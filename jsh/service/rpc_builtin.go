package service

import (
	"encoding/base64"
	"errors"
	"fmt"
)

func controllerBuiltinRPCMethods(ctl *Controller) map[string]any {
	return map[string]any{
		"service.list":                  ctl.rpcServiceList,
		"service.get":                   ctl.rpcServiceGet,
		"service.runtime.get":           ctl.rpcServiceRuntimeGet,
		"service.runtime.detail.add":    ctl.rpcServiceRuntimeDetailAdd,
		"service.runtime.detail.update": ctl.rpcServiceRuntimeDetailUpdate,
		"service.runtime.detail.set":    ctl.rpcServiceRuntimeDetailSet,
		"service.runtime.detail.delete": ctl.rpcServiceRuntimeDetailDelete,
		"fs.stat":                       ctl.rpcFSStat,
		"fs.readDir":                    ctl.rpcFSReadDir,
		"fs.readFile":                   ctl.rpcFSReadFile,
		"fs.writeFile":                  ctl.rpcFSWriteFile,
		"fs.chmod":                      ctl.rpcFSChmod,
		"fs.chown":                      ctl.rpcFSChown,
		"fs.mkdir":                      ctl.rpcFSMkdir,
		"fs.remove":                     ctl.rpcFSRemove,
		"fs.rename":                     ctl.rpcFSRename,
		"fs.open":                       ctl.rpcFSOpen,
		"fs.read":                       ctl.rpcFSRead,
		"fs.write":                      ctl.rpcFSWrite,
		"fs.close":                      ctl.rpcFSClose,
		"fs.fstat":                      ctl.rpcFSFstat,
		"fs.fsync":                      ctl.rpcFSFsync,
		"fs.fchmod":                     ctl.rpcFSFchmod,
		"fs.fchown":                     ctl.rpcFSFchown,
		"service.read":                  ctl.rpcServiceRead,
		"service.update":                ctl.rpcServiceUpdate,
		"service.reload":                ctl.rpcServiceReload,
		"service.install":               ctl.rpcServiceInstall,
		"service.uninstall":             ctl.rpcServiceUninstall,
		"service.start":                 ctl.rpcServiceStart,
		"service.stop":                  ctl.rpcServiceStop,
		"controller.metrics.get":        ctl.rpcControllerMetricsGet,
		"controller.metrics.reset":      ctl.rpcControllerMetricsReset,
		"llm.session.open":              ctl.rpcLLMSessionOpen,
		"llm.session.get":               ctl.rpcLLMSessionGet,
		"llm.session.reset":             ctl.rpcLLMSessionReset,
		"llm.turn.ask":                  ctl.rpcLLMTurnAsk,
		"llm.turn.cancel":               ctl.rpcLLMTurnCancel,
		"llm.provider.set":              ctl.rpcLLMProviderSet,
		"llm.model.set":                 ctl.rpcLLMModelSet,
	}
}

func (ctl *Controller) rpcServiceList() []ServiceSnapshot {
	return ctl.statusSnapshots()
}

func (ctl *Controller) rpcServiceGet(req serviceNameRequest) (ServiceSnapshot, error) {
	svc, err := ctl.requireService(req.Name)
	if err != nil {
		return ServiceSnapshot{}, err
	}
	return snapshotService(svc), nil
}

func (ctl *Controller) rpcServiceRuntimeGet(req serviceNameRequest) (ServiceRuntimeSnapshot, error) {
	svc, err := ctl.requireService(req.Name)
	if err != nil {
		return ServiceRuntimeSnapshot{}, err
	}
	return snapshotServiceRuntime(svc), nil
}

func (ctl *Controller) rpcServiceRuntimeDetailAdd(req serviceRuntimeDetailRequest) (ServiceRuntimeSnapshot, error) {
	svc, err := ctl.requireService(req.Name)
	if err != nil {
		return ServiceRuntimeSnapshot{}, err
	}
	if err := svc.addDetail(req.Key, req.Value); err != nil {
		return ServiceRuntimeSnapshot{}, invalidParamsError(err)
	}
	return snapshotServiceRuntime(svc), nil
}

func (ctl *Controller) rpcServiceRuntimeDetailUpdate(req serviceRuntimeDetailRequest) (ServiceRuntimeSnapshot, error) {
	svc, err := ctl.requireService(req.Name)
	if err != nil {
		return ServiceRuntimeSnapshot{}, err
	}
	if err := svc.updateDetail(req.Key, req.Value); err != nil {
		return ServiceRuntimeSnapshot{}, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
	}
	return snapshotServiceRuntime(svc), nil
}

func (ctl *Controller) rpcServiceRuntimeDetailSet(req serviceRuntimeDetailRequest) (ServiceRuntimeSnapshot, error) {
	svc, err := ctl.requireService(req.Name)
	if err != nil {
		return ServiceRuntimeSnapshot{}, err
	}
	if err := svc.setDetail(req.Key, req.Value); err != nil {
		return ServiceRuntimeSnapshot{}, invalidParamsError(err)
	}
	return snapshotServiceRuntime(svc), nil
}

func (ctl *Controller) rpcServiceRuntimeDetailDelete(req serviceRuntimeDetailRequest) (ServiceRuntimeSnapshot, error) {
	svc, err := ctl.requireService(req.Name)
	if err != nil {
		return ServiceRuntimeSnapshot{}, err
	}
	if err := svc.deleteDetail(req.Key); err != nil {
		return ServiceRuntimeSnapshot{}, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
	}
	return snapshotServiceRuntime(svc), nil
}

func (ctl *Controller) rpcControllerMetricsGet() ControllerRPCMetricsSnapshot {
	return ctl.rpcMetricsSnapshot()
}

func (ctl *Controller) rpcControllerMetricsReset() ControllerRPCMetricsSnapshot {
	return ctl.resetRPCMetrics()
}

func (ctl *Controller) rpcFSStat(req sharedPathRequest) (SharedFileInfoSnapshot, error) {
	info, err := ctl.sharedStat(req.Path)
	if err != nil {
		return SharedFileInfoSnapshot{}, mapSharedFSError(err)
	}
	return info, nil
}

func (ctl *Controller) rpcFSReadDir(req sharedPathRequest) ([]SharedFileInfoSnapshot, error) {
	entries, err := ctl.sharedReadDir(req.Path)
	if err != nil {
		return nil, mapSharedFSError(err)
	}
	return entries, nil
}

func (ctl *Controller) rpcFSReadFile(req sharedPathRequest) (SharedReadFileResult, error) {
	result, err := ctl.sharedReadFileRPC(req.Path)
	if err != nil {
		return SharedReadFileResult{}, mapSharedFSError(err)
	}
	return result, nil
}

func (ctl *Controller) rpcFSWriteFile(req sharedWriteFileRequest) (SharedFileInfoSnapshot, error) {
	info, err := ctl.sharedWriteFileRPC(req.Path, req.Data)
	if err != nil {
		var base64Err base64.CorruptInputError
		if errors.As(err, &base64Err) {
			return SharedFileInfoSnapshot{}, invalidParamsError(err)
		}
		return SharedFileInfoSnapshot{}, mapSharedFSError(err)
	}
	return info, nil
}

func (ctl *Controller) rpcFSChmod(req sharedChmodRequest) (bool, error) {
	if err := ctl.sharedChmod(req.Path, req.Mode); err != nil {
		return false, mapSharedFSError(err)
	}
	return true, nil
}

func (ctl *Controller) rpcFSChown(req sharedChownRequest) (bool, error) {
	if err := ctl.sharedChown(req.Path, req.UID, req.GID); err != nil {
		return false, mapSharedFSError(err)
	}
	return true, nil
}

func (ctl *Controller) rpcFSMkdir(req sharedPathRequest) (SharedFileInfoSnapshot, error) {
	if err := ctl.sharedMkdir(req.Path); err != nil {
		return SharedFileInfoSnapshot{}, mapSharedFSError(err)
	}
	info, err := ctl.sharedStat(req.Path)
	if err != nil {
		return SharedFileInfoSnapshot{}, mapSharedFSError(err)
	}
	return info, nil
}

func (ctl *Controller) rpcFSRemove(req sharedPathRequest) (bool, error) {
	if err := ctl.sharedRemove(req.Path); err != nil {
		return false, mapSharedFSError(err)
	}
	return true, nil
}

func (ctl *Controller) rpcFSRename(req sharedRenameRequest) (SharedFileInfoSnapshot, error) {
	if err := ctl.sharedRename(req.OldPath, req.NewPath); err != nil {
		return SharedFileInfoSnapshot{}, mapSharedFSError(err)
	}
	info, err := ctl.sharedStat(req.NewPath)
	if err != nil {
		return SharedFileInfoSnapshot{}, mapSharedFSError(err)
	}
	return info, nil
}

func (ctl *Controller) rpcFSOpen(req sharedOpenFDRequest) (SharedOpenFDResult, error) {
	result, err := ctl.sharedOpenFD(req.Path, req.Flags, req.Mode, req.Owner)
	if err != nil {
		return SharedOpenFDResult{}, mapSharedFSError(err)
	}
	return result, nil
}

func (ctl *Controller) rpcFSRead(req sharedReadFDRequest) (SharedReadFDResult, error) {
	result, err := ctl.sharedReadFD(req.FD, req.Length)
	if err != nil {
		return SharedReadFDResult{}, mapSharedFSError(err)
	}
	return result, nil
}

func (ctl *Controller) rpcFSWrite(req sharedWriteFDRequest) (SharedWriteFDResult, error) {
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return SharedWriteFDResult{}, invalidParamsError(err)
	}
	result, err := ctl.sharedWriteFD(req.FD, data)
	if err != nil {
		return SharedWriteFDResult{}, mapSharedFSError(err)
	}
	return result, nil
}

func (ctl *Controller) rpcFSClose(req sharedFDRequest) (bool, error) {
	if err := ctl.sharedCloseFD(req.FD); err != nil {
		return false, mapSharedFSError(err)
	}
	return true, nil
}

func (ctl *Controller) rpcFSFstat(req sharedFDRequest) (SharedFileInfoSnapshot, error) {
	result, err := ctl.sharedFstatFD(req.FD)
	if err != nil {
		return SharedFileInfoSnapshot{}, mapSharedFSError(err)
	}
	return result, nil
}

func (ctl *Controller) rpcFSFsync(req sharedFDRequest) (bool, error) {
	if err := ctl.sharedFsyncFD(req.FD); err != nil {
		return false, mapSharedFSError(err)
	}
	return true, nil
}

func (ctl *Controller) rpcFSFchmod(req sharedFchmodFDRequest) (bool, error) {
	if err := ctl.sharedFchmodFD(req.FD, req.Mode); err != nil {
		return false, mapSharedFSError(err)
	}
	return true, nil
}

func (ctl *Controller) rpcFSFchown(req sharedFchownFDRequest) (bool, error) {
	if err := ctl.sharedFchownFD(req.FD, req.UID, req.GID); err != nil {
		return false, mapSharedFSError(err)
	}
	return true, nil
}

func (ctl *Controller) rpcServiceRead() (ServiceListSnapshot, error) {
	if err := ctl.Read(); err != nil {
		return ServiceListSnapshot{}, internalRPCError(err)
	}
	return ctl.rereadSnapshot(), nil
}

func (ctl *Controller) rpcServiceUpdate() ControllerUpdateResult {
	return ctl.updateSnapshot()
}

func (ctl *Controller) rpcServiceReload() (ControllerUpdateResult, error) {
	if err := ctl.Read(); err != nil {
		return ControllerUpdateResult{}, internalRPCError(err)
	}
	return ctl.reloadSnapshot(), nil
}

func (ctl *Controller) rpcServiceInstall(sc Config) (ServiceSnapshot, error) {
	if err := ctl.Install(&sc); err != nil {
		return ServiceSnapshot{}, internalRPCError(err)
	}
	svc := ctl.StatusOf(sc.Name)
	if svc == nil {
		return ServiceSnapshot{}, &controllerRPCError{Code: jsonRPCInternal, Message: fmt.Sprintf("service %s missing after install", sc.Name)}
	}
	return snapshotService(svc), nil
}

func (ctl *Controller) rpcServiceUninstall(req serviceNameRequest) (bool, error) {
	if err := ctl.Uninstall(req.Name); err != nil {
		return false, internalRPCError(err)
	}
	return true, nil
}

func (ctl *Controller) rpcServiceStart(req serviceNameRequest) (ServiceSnapshot, error) {
	svc, err := ctl.StartService(req.Name)
	if err != nil {
		if svc == nil {
			return ServiceSnapshot{}, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
		}
		return ServiceSnapshot{}, internalRPCError(err)
	}
	return snapshotService(svc), nil
}

func (ctl *Controller) rpcServiceStop(req serviceNameRequest) (ServiceSnapshot, error) {
	svc, err := ctl.StopService(req.Name)
	if err != nil {
		if svc == nil {
			return ServiceSnapshot{}, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
		}
		return ServiceSnapshot{}, internalRPCError(err)
	}
	return snapshotService(svc), nil
}

func (ctl *Controller) requireService(name string) (*Service, error) {
	ctl.mu.RLock()
	svc, exists := ctl.services[name]
	ctl.mu.RUnlock()
	if !exists || svc == nil {
		return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: fmt.Sprintf("service %s not found", name)}
	}
	return svc, nil
}
