package qwen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	httpclient "github.com/tmc/langchaingo/llms/tongyi/internal/tongyiclient/httpclient"
)

//nolint:lll
func AsyncParseStreamingChatResponse[T IQwenContent](ctx context.Context, payload *Request[T], cli httpclient.IHttpClient, token string) (*OutputResponse[T], error) {
	if payload.Model == "" {
		return nil, ErrModelNotSet
	}
	responseChan := asyncChatStreaming(ctx, payload, cli, token)
	outputMessage := OutputResponse[T]{}
	for rspData := range responseChan {
		if rspData.Err != nil {
			return nil, &httpclient.HTTPRequestError{Message: "parseStreamingChatResponse failed", Cause: rspData.Err}
		}
		if len(rspData.Output.Output.Choices) == 0 {
			return nil, ErrEmptyResponse
		}

		chunk := rspData.Output.Output.Choices[0].Message.Content.ToBytes()

		if payload.StreamingFunc != nil {
			err := payload.StreamingFunc(ctx, chunk)
			if err != nil {
				return nil, &httpclient.HTTPRequestError{Message: "parseStreamingChatResponse failed", Cause: err}
			}
		}

		outputMessage.RequestID = rspData.Output.RequestID
		outputMessage.Usage = rspData.Output.Usage
		if outputMessage.Output.Choices == nil {
			outputMessage.Output.Choices = rspData.Output.Output.Choices
		} else {
			choice := outputMessage.Output.Choices[0]
			choice.Message.Role = rspData.Output.Output.Choices[0].Message.Role
			choice.Message.Content.AppendText(rspData.Output.Output.Choices[0].Message.Content.ToString())
			choice.FinishReason = rspData.Output.Output.Choices[0].FinishReason

			outputMessage.Output.Choices[0] = choice
		}
	}

	return &outputMessage, nil
}

//nolint:lll
func SyncCall[T IQwenContent](ctx context.Context, payload *Request[T], cli httpclient.IHttpClient, token string) (*OutputResponse[T], error) {
	if payload.Model == "" {
		return nil, ErrModelNotSet
	}

	resp := OutputResponse[T]{}
	tokenOpt := httpclient.WithTokenHeaderOption(token)

	// FIXME: 临时处理，后续需要统一
	url := payload.Input.Messages[0].Content.TargetURL()
	err := cli.Post(ctx, url, payload, &resp, tokenOpt)
	if err != nil {
		return nil, err
	}
	if len(resp.Output.Choices) == 0 {
		return nil, ErrEmptyResponse
	}
	return &resp, nil
}

//nolint:lll
func asyncChatStreaming[T IQwenContent](ctx context.Context, r *Request[T], cli httpclient.IHttpClient, token string) <-chan StreamOutput[T] {
	chanBuffer := 100
	_respChunkChannel := make(chan StreamOutput[T], chanBuffer)

	go func() {
		withHeader := map[string]string{
			"Accept": "text/event-stream",
		}

		_combineStreamingChunk(ctx, r, withHeader, _respChunkChannel, cli, token)
	}()
	return _respChunkChannel
}

/*
 * combine SSE streaming lines to be a structed response data
 * id: xxxx
 * event: xxxxx
 * ......
 */
func _combineStreamingChunk[T IQwenContent](
	ctx context.Context,
	payload *Request[T],
	withHeader map[string]string,
	_respChunkChannel chan StreamOutput[T],
	cli httpclient.IHttpClient,
	token string,
) {
	defer close(_respChunkChannel)
	var _rawStreamOutChannel chan string

	var err error
	headerOpt := httpclient.WithHeader(withHeader)
	tokenOpt := httpclient.WithTokenHeaderOption(token)

	// FIXME: 临时处理，后续需要统一
	url := payload.Input.Messages[0].Content.TargetURL()
	_rawStreamOutChannel, err = cli.PostSSE(ctx, url, payload, headerOpt, tokenOpt)

	if err != nil {
		_respChunkChannel <- StreamOutput[T]{Err: err}
		return
	}

	rsp := StreamOutput[T]{}

	for v := range _rawStreamOutChannel {
		if strings.TrimSpace(v) == "" {
			// streaming out combined response
			_respChunkChannel <- rsp
			rsp = StreamOutput[T]{}
			continue
		}

		err = fillInRespData(v, &rsp)
		if err != nil {
			rsp.Err = err
			_respChunkChannel <- rsp
			break
		}
	}
}

// filled in response data line by line.
func fillInRespData[T IQwenContent](line string, output *StreamOutput[T]) error {
	if strings.TrimSpace(line) == "" {
		return nil
	}

	switch {
	case strings.HasPrefix(line, "id:"):
		output.ID = strings.TrimPrefix(line, "id:")
	case strings.HasPrefix(line, "event:"):
		output.Event = strings.TrimPrefix(line, "event:")
	case strings.HasPrefix(line, ":HTTP_STATUS/"):
		code, err := strconv.Atoi(strings.TrimPrefix(line, ":HTTP_STATUS/"))
		if err != nil {
			output.Err = fmt.Errorf("http_status err: strconv.Atoi  %w", err)
		}
		output.HTTPStatus = code
	case strings.HasPrefix(line, "data:"):
		dataJSON := strings.TrimPrefix(line, "data:")
		if output.Event == "error" {
			output.Err = &WrapMessageError{Message: dataJSON}
			return nil
		}
		outputData := OutputResponse[T]{}
		err := json.Unmarshal([]byte(dataJSON), &outputData)
		if err != nil {
			return &WrapMessageError{Message: "unmarshal OutputData Err", Cause: err}
		}

		output.Output = outputData
	default:
		data := bytes.TrimSpace([]byte(line))
		log.Printf("unknown line: %s", data)
	}

	return nil
}
