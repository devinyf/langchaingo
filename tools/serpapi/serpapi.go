package serpapi

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/tools"
	"github.com/tmc/langchaingo/tools/serpapi/internal"
)

var ErrMissingToken = errors.New("missing the serpapi API key, set it in the SERPAPI_API_KEY environment variable")

type Tool struct {
	CallbacksHandler callbacks.Handler
	client           *internal.Client
}

var _ tools.Tool = Tool{}

// New creates a new serpapi tool to search on internet.
func New() (*Tool, error) {
	apiKey := os.Getenv("SERPAPI_API_KEY")
	if apiKey == "" {
		return nil, ErrMissingToken
	}

	return &Tool{
		client: internal.New(apiKey),
	}, nil
}

func (t Tool) Name() string {
	return "GoogleSearch"
}

func (t Tool) Description() string {
	return "A search engine. Useful for when you need to answer questions about current events. Input should be a search query."
}

func (t Tool) Call(ctx context.Context, input string) (string, error) {
	if t.CallbacksHandler != nil {
		t.CallbacksHandler.HandleToolStart(ctx, input)
	}

	result, err := t.client.Search(ctx, input)
	if err != nil {
		if errors.Is(err, internal.ErrNoGoodResult) {
			return "No good Google Search Results was found", nil
		}

		if t.CallbacksHandler != nil {
			t.CallbacksHandler.HandleToolError(ctx, err)
		}

		return "", err
	}

	if t.CallbacksHandler != nil {
		t.CallbacksHandler.HandleToolEnd(ctx, result)
	}

	return strings.Join(strings.Fields(result), " "), nil
}
