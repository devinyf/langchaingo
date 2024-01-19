package tongyiclient

import (
	"context"

	embedding "github.com/tmc/langchaingo/llms/tongyi/internal/tongyiclient/embedding"
	httpclient "github.com/tmc/langchaingo/llms/tongyi/internal/tongyiclient/httpclient"
	qwen "github.com/tmc/langchaingo/llms/tongyi/internal/tongyiclient/qwen"
	wanx "github.com/tmc/langchaingo/llms/tongyi/internal/tongyiclient/wanx"
)

type TongyiClient struct {
	Model   string
	token   string
	httpCli httpclient.IHttpClient
}

func NewTongyiClient(model string, token string) *TongyiClient {
	httpcli := httpclient.NewHTTPClient()
	return newTongyiCLientWithHttpCli(model, token, httpcli)
}

func newTongyiCLientWithHttpCli(model string, token string, httpcli httpclient.IHttpClient) *TongyiClient {
	return &TongyiClient{
		Model:   model,
		httpCli: httpcli,
		token:   token,
	}
}

func (q *TongyiClient) CreateCompletion(ctx context.Context, payload *qwen.QwenRequest[*qwen.TextContent]) (*TextQwenOutputMessage, error) {
	if payload.Model == "" {
		payload.Model = q.Model
	}
	if payload.Parameters == nil {
		payload.Parameters = qwen.DefaultParameters()
	}
	return genericCompletion(ctx, payload, q.httpCli, q.token)
}

func (q *TongyiClient) CreateVLCompletion(ctx context.Context, payload *qwen.QwenRequest[*qwen.VLContentList]) (*VLQwenOutputMessage, error) {
	if payload.Model == "" {
		payload.Model = q.Model
	}

	if payload.Parameters == nil {
		payload.Parameters = &qwen.Parameters{}
	}

	return genericCompletion(ctx, payload, q.httpCli, q.token)
}

func genericCompletion[T qwen.IQwenContent](ctx context.Context, payload *qwen.QwenRequest[T], httpcli httpclient.IHttpClient, token string) (*qwen.QwenOutputMessage[T], error) {
	if payload.Model == "" {
		return nil, ErrModelNotSet
	}

	if payload.StreamingFunc != nil {
		payload.Parameters.SetIncrementalOutput(true)
		return qwen.AsyncParseStreamingChatResponse(ctx, payload, httpcli, token)
	}
	return qwen.SyncCall(ctx, payload, httpcli, token)
}

func (q *TongyiClient) CreateImageGeneration(ctx context.Context, payload *wanx.WanxImageSynthesisRequest) ([]*wanx.WanxImgBlob, error) {
	if payload.Model == "" {
		if q.Model == "" {
			return nil, ErrModelNotSet
		}
		payload.Model = q.Model
	}
	return wanx.CreateImageGeneration(ctx, payload, q.httpCli, q.token)
}

func (q *TongyiClient) CreateEmbedding(ctx context.Context, r *embedding.EmbeddingRequest) ([][]float32, error) {

	resp, err := embedding.CreateEmbedding(ctx, r, q.httpCli, q.token)
	if err != nil {
		return nil, err
	}
	if len(resp.Output.Embeddings) == 0 {
		return nil, ErrEmptyResponse
	}

	embeddings := make([][]float32, 0)
	for i := 0; i < len(resp.Output.Embeddings); i++ {
		embeddings = append(embeddings, resp.Output.Embeddings[i].Embedding)
	}
	return embeddings, nil
}
