package dbus

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/dop251/goja"
	godbus "github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

//go:embed dbus.js
var dbus_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"dbus.js": dbus_js,
	}
}

func Module(ctx context.Context, rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/dbus")
	o := module.Get("exports").(*goja.Object)
	o.Set("newConnection", func(obj *goja.Object, dispatch engine.EventDispatchFunc, opts ConnectionOptions) (*Connection, error) {
		return NewConnection(ctx, obj, dispatch, opts)
	})
	o.Set("BusType", rt.ToValue(map[string]any{
		"Session": BusTypeSession,
		"System":  BusTypeSystem,
	}))
}

const (
	BusTypeSession = "session"
	BusTypeSystem  = "system"
)

type ConnectionOptions struct {
	BusType string `json:"busType"`
}

type CallRequest struct {
	Destination string       `json:"destination"`
	Path        string       `json:"path"`
	Method      string       `json:"method"`
	Args        []any        `json:"args"`
	Flags       godbus.Flags `json:"flags"`
}

type CallResult struct {
	Destination string `json:"destination"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	Body        []any  `json:"body"`
}

type PropertyRequest struct {
	Destination string `json:"destination"`
	Path        string `json:"path"`
	Interface   string `json:"interface"`
	Name        string `json:"name"`
	Property    string `json:"property"`
}

type PropertyResult struct {
	Signature string `json:"signature"`
	Value     any    `json:"value"`
}

type SetPropertyRequest struct {
	Destination string `json:"destination"`
	Path        string `json:"path"`
	Interface   string `json:"interface"`
	Name        string `json:"name"`
	Property    string `json:"property"`
	Value       any    `json:"value"`
}

type IntrospectRequest struct {
	Destination string `json:"destination"`
	Path        string `json:"path"`
}

type IntrospectionNode struct {
	Name       string                   `json:"name"`
	Interfaces []IntrospectionInterface `json:"interfaces"`
	Children   []IntrospectionChild     `json:"children"`
}

type IntrospectionInterface struct {
	Name        string                    `json:"name"`
	Methods     []IntrospectionMethod     `json:"methods"`
	Signals     []IntrospectionSignal     `json:"signals"`
	Properties  []IntrospectionProperty   `json:"properties"`
	Annotations []IntrospectionAnnotation `json:"annotations"`
}

type IntrospectionMethod struct {
	Name        string                    `json:"name"`
	Args        []IntrospectionArgument   `json:"args"`
	Annotations []IntrospectionAnnotation `json:"annotations"`
}

type IntrospectionSignal struct {
	Name        string                    `json:"name"`
	Args        []IntrospectionArgument   `json:"args"`
	Annotations []IntrospectionAnnotation `json:"annotations"`
}

type IntrospectionProperty struct {
	Name        string                    `json:"name"`
	Type        string                    `json:"type"`
	Access      string                    `json:"access"`
	Annotations []IntrospectionAnnotation `json:"annotations"`
}

type IntrospectionArgument struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Direction string `json:"direction"`
}

type IntrospectionAnnotation struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type IntrospectionChild struct {
	Name string `json:"name"`
}

type SignalWatchRequest struct {
	Destination string `json:"destination"`
	Sender      string `json:"sender"`
	Path        string `json:"path"`
	Interface   string `json:"interface"`
	Member      string `json:"member"`
}

type NameWatchRequest struct {
	Name string `json:"name"`
}

type NameOwnerRequest struct {
	Name string `json:"name"`
}

type NameOwnerResult struct {
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	HasOwner bool   `json:"hasOwner"`
}

type signalWatchState struct {
	request SignalWatchRequest
	count   int
}

type Connection struct {
	ctx     context.Context
	conn    *godbus.Conn
	busType string

	emit          func(event string, args ...any) bool
	signalCh      chan *godbus.Signal
	signalMu      sync.Mutex
	signalWatches map[string]signalWatchState
	nameWatches   map[string]int
}

func NewConnection(ctx context.Context, obj *goja.Object, dispatch engine.EventDispatchFunc, opts ConnectionOptions) (*Connection, error) {
	if err := ensureSupportedPlatform(runtime.GOOS); err != nil {
		return nil, err
	}

	busType := strings.ToLower(strings.TrimSpace(opts.BusType))
	if busType == "" {
		busType = BusTypeSession
	}

	var (
		conn *godbus.Conn
		err  error
	)
	switch busType {
	case BusTypeSession:
		conn, err = godbus.ConnectSessionBus()
	case BusTypeSystem:
		conn, err = godbus.ConnectSystemBus()
	default:
		return nil, fmt.Errorf("invalid bus type: %q", opts.BusType)
	}
	if err != nil {
		return nil, err
	}

	ret := &Connection{
		ctx:           ctx,
		conn:          conn,
		busType:       busType,
		signalWatches: map[string]signalWatchState{},
		nameWatches:   map[string]int{},
	}
	if obj != nil && dispatch != nil {
		ret.emit = func(event string, args ...any) bool {
			return dispatch(obj, event, args...)
		}
		ret.signalCh = make(chan *godbus.Signal, 16)
		ret.conn.Signal(ret.signalCh)
		go ret.signalLoop()
	}
	return ret, nil
}

func (c *Connection) Close() error {
	c.signalMu.Lock()
	if c.conn != nil && c.signalCh != nil {
		for _, watch := range c.signalWatches {
			_ = c.conn.RemoveMatchSignal(matchOptions(watch.request)...)
		}
		for name := range c.nameWatches {
			_ = c.conn.RemoveMatchSignal(nameWatchMatchOptions(name)...)
		}
		c.conn.RemoveSignal(c.signalCh)
		c.signalWatches = map[string]signalWatchState{}
		c.nameWatches = map[string]int{}
	}
	c.signalMu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	return nil
}

func (c *Connection) Call(request CallRequest) (*CallResult, error) {
	if err := c.validateCallRequest(request); err != nil {
		return nil, err
	}
	call := c.conn.Object(request.Destination, godbus.ObjectPath(request.Path)).CallWithContext(c.ctx, request.Method, request.Flags, request.Args...)
	if call.Err != nil {
		return nil, call.Err
	}
	body := make([]any, len(call.Body))
	for i, v := range call.Body {
		body[i] = normalizeValue(v)
	}
	return &CallResult{
		Destination: request.Destination,
		Path:        request.Path,
		Method:      request.Method,
		Body:        body,
	}, nil
}

func (c *Connection) GetProperty(request PropertyRequest) (*PropertyResult, error) {
	if err := c.validateObjectRequest(request.Destination, request.Path); err != nil {
		return nil, err
	}
	propertyName, err := propertyName(request.Interface, request.Name, request.Property)
	if err != nil {
		return nil, err
	}
	variant, err := c.conn.Object(request.Destination, godbus.ObjectPath(request.Path)).GetProperty(propertyName)
	if err != nil {
		return nil, err
	}
	return &PropertyResult{
		Signature: variant.Signature().String(),
		Value:     normalizeValue(variant.Value()),
	}, nil
}

func (c *Connection) SetProperty(request SetPropertyRequest) error {
	if err := c.validateObjectRequest(request.Destination, request.Path); err != nil {
		return err
	}
	propertyName, err := propertyName(request.Interface, request.Name, request.Property)
	if err != nil {
		return err
	}
	return c.conn.Object(request.Destination, godbus.ObjectPath(request.Path)).SetProperty(propertyName, godbus.MakeVariant(request.Value))
}

func (c *Connection) Introspect(request IntrospectRequest) (*IntrospectionNode, error) {
	if err := c.validateObjectRequest(request.Destination, request.Path); err != nil {
		return nil, err
	}
	node, err := introspect.Call(c.conn.Object(request.Destination, godbus.ObjectPath(request.Path)))
	if err != nil {
		return nil, err
	}
	result := toIntrospectionNode(node)
	return &result, nil
}

func (c *Connection) SubscribeSignal(request SignalWatchRequest) error {
	if err := c.requireSignalDispatch(); err != nil {
		return err
	}
	if err := validateSignalWatchRequest(request); err != nil {
		return err
	}

	key := signalWatchKey(request)
	options := matchOptions(request)

	c.signalMu.Lock()
	defer c.signalMu.Unlock()
	watch := c.signalWatches[key]
	if watch.count == 0 {
		if err := c.conn.AddMatchSignal(options...); err != nil {
			return err
		}
		watch.request = normalizedSignalWatchRequest(request)
	}
	watch.count++
	c.signalWatches[key] = watch
	return nil
}

func (c *Connection) UnsubscribeSignal(request SignalWatchRequest) error {
	if err := c.requireSignalDispatch(); err != nil {
		return err
	}
	if err := validateSignalWatchRequest(request); err != nil {
		return err
	}

	key := signalWatchKey(request)
	options := matchOptions(request)

	c.signalMu.Lock()
	defer c.signalMu.Unlock()
	watch, ok := c.signalWatches[key]
	if !ok || watch.count == 0 {
		return errors.New("signal subscription not found")
	}
	watch.count--
	if watch.count == 0 {
		delete(c.signalWatches, key)
		return c.conn.RemoveMatchSignal(options...)
	}
	c.signalWatches[key] = watch
	return nil
}

func (c *Connection) WatchName(request NameWatchRequest) error {
	if err := c.requireSignalDispatch(); err != nil {
		return err
	}
	name, err := normalizedWatchName(request)
	if err != nil {
		return err
	}

	c.signalMu.Lock()
	defer c.signalMu.Unlock()
	count := c.nameWatches[name]
	if count == 0 {
		if err := c.conn.AddMatchSignal(nameWatchMatchOptions(name)...); err != nil {
			return err
		}
	}
	c.nameWatches[name] = count + 1
	return nil
}

func (c *Connection) UnwatchName(request NameWatchRequest) error {
	if err := c.requireSignalDispatch(); err != nil {
		return err
	}
	name, err := normalizedWatchName(request)
	if err != nil {
		return err
	}

	c.signalMu.Lock()
	defer c.signalMu.Unlock()
	count := c.nameWatches[name]
	if count == 0 {
		return errors.New("name watch not found")
	}
	count--
	if count == 0 {
		delete(c.nameWatches, name)
		return c.conn.RemoveMatchSignal(nameWatchMatchOptions(name)...)
	}
	c.nameWatches[name] = count
	return nil
}

func (c *Connection) GetNameOwner(request NameOwnerRequest) (*NameOwnerResult, error) {
	if err := c.requireOpen(); err != nil {
		return nil, err
	}
	name, err := normalizedWatchName(NameWatchRequest{Name: request.Name})
	if err != nil {
		return nil, err
	}

	call := c.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").CallWithContext(c.ctx, "org.freedesktop.DBus.GetNameOwner", 0, name)
	if call.Err != nil {
		if isNameHasNoOwnerError(call.Err) {
			return &NameOwnerResult{Name: name, Owner: "", HasOwner: false}, nil
		}
		return nil, call.Err
	}

	var owner string
	if err := call.Store(&owner); err != nil {
		return nil, err
	}
	return &NameOwnerResult{Name: name, Owner: owner, HasOwner: owner != ""}, nil
}

func (c *Connection) validateCallRequest(request CallRequest) error {
	if err := c.requireOpen(); err != nil {
		return err
	}
	if err := c.validateObjectRequest(request.Destination, request.Path); err != nil {
		return err
	}
	if strings.TrimSpace(request.Method) == "" {
		return errors.New("missing method")
	}
	return nil
}

func (c *Connection) validateObjectRequest(destination, path string) error {
	if err := c.requireOpen(); err != nil {
		return err
	}
	if strings.TrimSpace(destination) == "" {
		return errors.New("missing destination")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("missing path")
	}
	if !godbus.ObjectPath(path).IsValid() {
		return fmt.Errorf("invalid object path: %q", path)
	}
	return nil
}

func (c *Connection) requireOpen() error {
	if c == nil || c.conn == nil {
		return errors.New("connection not initialized")
	}
	return nil
}

func (c *Connection) requireSignalDispatch() error {
	if err := c.requireOpen(); err != nil {
		return err
	}
	if c.emit == nil || c.signalCh == nil {
		return errors.New("signal dispatch not initialized")
	}
	return nil
}

func (c *Connection) signalLoop() {
	for sig := range c.signalCh {
		if sig == nil || c.emit == nil {
			continue
		}
		event := newSignalEvent(sig)
		c.emit("signal", event)
		if ownerChange, ok := newNameOwnerChangedEvent(event); ok {
			c.emit("name-owner-changed", ownerChange)
		}
	}
}

func ensureSupportedPlatform(goos string) error {
	if goos != "linux" {
		return fmt.Errorf("dbus is supported only on linux")
	}
	return nil
}

func propertyName(iface, name, property string) (string, error) {
	property = strings.TrimSpace(property)
	if property != "" {
		return property, nil
	}
	iface = strings.TrimSpace(iface)
	name = strings.TrimSpace(name)
	if iface == "" || name == "" {
		return "", errors.New("missing property")
	}
	return iface + "." + name, nil
}

func validateSignalWatchRequest(request SignalWatchRequest) error {
	sender := strings.TrimSpace(request.Sender)
	path := strings.TrimSpace(request.Path)
	iface := strings.TrimSpace(request.Interface)
	member := strings.TrimSpace(request.Member)

	if sender == "" && path == "" && iface == "" && member == "" {
		return errors.New("missing signal match criteria")
	}
	if path != "" && !godbus.ObjectPath(path).IsValid() {
		return fmt.Errorf("invalid object path: %q", path)
	}
	return nil
}

func normalizedWatchName(request NameWatchRequest) (string, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return "", errors.New("missing name")
	}
	return name, nil
}

func isNameHasNoOwnerError(err error) bool {
	var dbusErr *godbus.Error
	if !errors.As(err, &dbusErr) {
		msg := strings.ToLower(err.Error())
		return strings.Contains(msg, "namehasnoowner") || strings.Contains(msg, "no such name") || strings.Contains(msg, "has no owner")
	}
	if dbusErr.Name == "org.freedesktop.DBus.Error.NameHasNoOwner" {
		return true
	}
	msg := strings.ToLower(dbusErr.Error())
	return strings.Contains(msg, "namehasnoowner") || strings.Contains(msg, "no such name") || strings.Contains(msg, "has no owner")
}

func normalizedSignalWatchRequest(request SignalWatchRequest) SignalWatchRequest {
	request.Destination = strings.TrimSpace(request.Destination)
	request.Sender = strings.TrimSpace(request.Sender)
	request.Path = strings.TrimSpace(request.Path)
	request.Interface = strings.TrimSpace(request.Interface)
	request.Member = strings.TrimSpace(request.Member)
	return request
}

func signalWatchKey(request SignalWatchRequest) string {
	request = normalizedSignalWatchRequest(request)
	return strings.Join([]string{request.Sender, request.Path, request.Interface, request.Member}, "\x00")
}

func matchOptions(request SignalWatchRequest) []godbus.MatchOption {
	request = normalizedSignalWatchRequest(request)
	options := make([]godbus.MatchOption, 0, 4)
	if request.Sender != "" {
		options = append(options, godbus.WithMatchSender(request.Sender))
	}
	if request.Path != "" {
		options = append(options, godbus.WithMatchObjectPath(godbus.ObjectPath(request.Path)))
	}
	if request.Interface != "" {
		options = append(options, godbus.WithMatchInterface(request.Interface))
	}
	if request.Member != "" {
		options = append(options, godbus.WithMatchMember(request.Member))
	}
	return options
}

func nameWatchMatchOptions(name string) []godbus.MatchOption {
	return []godbus.MatchOption{
		godbus.WithMatchSender("org.freedesktop.DBus"),
		godbus.WithMatchObjectPath(godbus.ObjectPath("/org/freedesktop/DBus")),
		godbus.WithMatchInterface("org.freedesktop.DBus"),
		godbus.WithMatchMember("NameOwnerChanged"),
		godbus.WithMatchArg(0, name),
	}
}

type signalEvent struct {
	Sender    string
	Path      string
	Name      string
	Interface string
	Member    string
	Body      []any
}

type nameOwnerChangedEvent struct {
	Name     string
	OldOwner string
	NewOwner string
}

func newSignalEvent(sig *godbus.Signal) signalEvent {
	iface := ""
	member := sig.Name
	if idx := strings.LastIndex(sig.Name, "."); idx >= 0 {
		iface = sig.Name[:idx]
		member = sig.Name[idx+1:]
	}
	body := make([]any, len(sig.Body))
	for i, item := range sig.Body {
		body[i] = normalizeValue(item)
	}
	return signalEvent{
		Sender:    sig.Sender,
		Path:      string(sig.Path),
		Name:      sig.Name,
		Interface: iface,
		Member:    member,
		Body:      body,
	}
}

func newNameOwnerChangedEvent(evt signalEvent) (nameOwnerChangedEvent, bool) {
	if evt.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(evt.Body) < 3 {
		return nameOwnerChangedEvent{}, false
	}
	name, ok1 := evt.Body[0].(string)
	oldOwner, ok2 := evt.Body[1].(string)
	newOwner, ok3 := evt.Body[2].(string)
	if !ok1 || !ok2 || !ok3 {
		return nameOwnerChangedEvent{}, false
	}
	return nameOwnerChangedEvent{Name: name, OldOwner: oldOwner, NewOwner: newOwner}, true
}

func toIntrospectionNode(node *introspect.Node) IntrospectionNode {
	if node == nil {
		return IntrospectionNode{}
	}
	interfaces := make([]IntrospectionInterface, 0, len(node.Interfaces))
	for _, iface := range node.Interfaces {
		interfaces = append(interfaces, toIntrospectionInterface(iface))
	}
	children := make([]IntrospectionChild, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, IntrospectionChild{Name: child.Name})
	}
	return IntrospectionNode{
		Name:       node.Name,
		Interfaces: interfaces,
		Children:   children,
	}
}

func toIntrospectionInterface(iface introspect.Interface) IntrospectionInterface {
	methods := make([]IntrospectionMethod, 0, len(iface.Methods))
	for _, method := range iface.Methods {
		methods = append(methods, IntrospectionMethod{
			Name:        method.Name,
			Args:        toIntrospectionArguments(method.Args),
			Annotations: toIntrospectionAnnotations(method.Annotations),
		})
	}
	signals := make([]IntrospectionSignal, 0, len(iface.Signals))
	for _, signal := range iface.Signals {
		signals = append(signals, IntrospectionSignal{
			Name:        signal.Name,
			Args:        toIntrospectionArguments(signal.Args),
			Annotations: toIntrospectionAnnotations(signal.Annotations),
		})
	}
	properties := make([]IntrospectionProperty, 0, len(iface.Properties))
	for _, property := range iface.Properties {
		properties = append(properties, IntrospectionProperty{
			Name:        property.Name,
			Type:        property.Type,
			Access:      property.Access,
			Annotations: toIntrospectionAnnotations(property.Annotations),
		})
	}
	return IntrospectionInterface{
		Name:        iface.Name,
		Methods:     methods,
		Signals:     signals,
		Properties:  properties,
		Annotations: toIntrospectionAnnotations(iface.Annotations),
	}
}

func toIntrospectionArguments(args []introspect.Arg) []IntrospectionArgument {
	ret := make([]IntrospectionArgument, 0, len(args))
	for _, arg := range args {
		ret = append(ret, IntrospectionArgument{
			Name:      arg.Name,
			Type:      arg.Type,
			Direction: arg.Direction,
		})
	}
	return ret
}

func toIntrospectionAnnotations(annotations []introspect.Annotation) []IntrospectionAnnotation {
	ret := make([]IntrospectionAnnotation, 0, len(annotations))
	for _, annotation := range annotations {
		ret = append(ret, IntrospectionAnnotation{
			Name:  annotation.Name,
			Value: annotation.Value,
		})
	}
	return ret
}

func (evt signalEvent) EventValue(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	obj.Set("sender", evt.Sender)
	obj.Set("path", evt.Path)
	obj.Set("name", evt.Name)
	obj.Set("interface", evt.Interface)
	obj.Set("member", evt.Member)
	obj.Set("body", evt.Body)
	return obj
}

func (evt nameOwnerChangedEvent) EventValue(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	obj.Set("name", evt.Name)
	obj.Set("oldOwner", evt.OldOwner)
	obj.Set("newOwner", evt.NewOwner)
	return obj
}

func normalizeValue(value any) any {
	switch v := value.(type) {
	case godbus.Variant:
		return normalizeValue(v.Value())
	case godbus.ObjectPath:
		return string(v)
	case godbus.Signature:
		return v.String()
	case []any:
		ret := make([]any, len(v))
		for i, item := range v {
			ret[i] = normalizeValue(item)
		}
		return ret
	case map[string]godbus.Variant:
		ret := make(map[string]any, len(v))
		for key, item := range v {
			ret[key] = normalizeValue(item.Value())
		}
		return ret
	default:
		return value
	}
}
