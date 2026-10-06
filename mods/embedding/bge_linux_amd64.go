//go:build linux && amd64

package embedding

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"

	"github.com/machbase/neo-client/v2/api"
	sp "github.com/tggo/goSentencePiece"
	ort "github.com/yalue/onnxruntime_go"
)

type bgeModel struct {
	tokenizer *sp.Tokenizer
	session   *ort.DynamicAdvancedSession
}

func (s *Service) embedBGE(ctx context.Context, text string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	model, err := s.loadBGE(ctx)
	if err != nil {
		return Result{}, err
	}
	encoding := model.tokenizer.EncodeWithOptions(text, true)
	if encoding == nil || len(encoding.IDs) < 2 || len(encoding.IDs) > s.config.MaxTokens || len(encoding.IDs) > 8192 {
		return Result{}, errors.New("BGE-M3 token count is out of range")
	}
	shape := ort.Shape{1, int64(len(encoding.IDs))}
	ids := make([]int64, len(encoding.IDs))
	mask := make([]int64, len(encoding.IDs))
	for i, id := range encoding.IDs {
		ids[i] = int64(id)
		mask[i] = int64(encoding.AttentionMask[i])
	}
	idTensor, err := ort.NewTensor(shape, ids)
	if err != nil {
		return Result{}, err
	}
	defer idTensor.Destroy()
	maskTensor, err := ort.NewTensor(shape, mask)
	if err != nil {
		return Result{}, err
	}
	defer maskTensor.Destroy()
	outputs := make([]ort.Value, 1)
	s.runMu.Lock()
	err = model.session.Run([]ort.Value{idTensor, maskTensor}, outputs)
	s.runMu.Unlock()
	if err != nil {
		return Result{}, err
	}
	defer outputs[0].Destroy()
	tensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return Result{}, fmt.Errorf("unexpected BGE-M3 output %T", outputs[0])
	}
	vector := append(api.Vector(nil), tensor.GetData()...)
	if len(vector) != BGEDimension {
		return Result{}, fmt.Errorf("BGE-M3 returned %d dimensions", len(vector))
	}
	if err := vector.Validate(); err != nil {
		return Result{}, err
	}
	norm := 0.0
	for _, value := range vector {
		norm += float64(value) * float64(value)
	}
	if math.Abs(math.Sqrt(norm)-1) > 1e-3 {
		return Result{}, errors.New("BGE-M3 output is not L2-normalized")
	}
	return Result{Vector: vector, Dimension: BGEDimension, Space: BGESpace, Model: BGEModel}, ctx.Err()
}

func (s *Service) loadBGE(ctx context.Context) (*bgeModel, error) {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	if s.model != nil {
		return s.model, nil
	}
	modelDir, runtimeLibrary, err := ensureAssets(ctx, s.config)
	if err != nil {
		return nil, err
	}
	ort.SetSharedLibraryPath(runtimeLibrary)
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("initialize ONNX Runtime: %w", err)
	}
	tokenizer, err := sp.NewTokenizerFromJSON(filepath.Join(modelDir, "tokenizer.json"))
	if err != nil {
		_ = ort.DestroyEnvironment()
		return nil, err
	}
	session, err := ort.NewDynamicAdvancedSession(filepath.Join(modelDir, "onnx", "model.onnx"),
		[]string{"input_ids", "attention_mask"}, []string{"sentence_embedding"}, nil)
	if err != nil {
		_ = ort.DestroyEnvironment()
		return nil, err
	}
	s.model = &bgeModel{tokenizer: tokenizer, session: session}
	return s.model, nil
}
