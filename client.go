package gozaya

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type GoZaya struct {
	basePath    string
	restyClient *resty.Client
	Config      struct {
		CreateLinkEndpoint string
		GetLinkEndpoint    string
	}
}

const (
	adminClientID string = "admin-cli"
	urlSeparator  string = "/"
)

func makeURL(path ...string) string {
	return strings.Join(path, urlSeparator)
}

// GetRequest returns a request for calling endpoints.
func (g *GoZaya) GetRequest(ctx context.Context) *resty.Request {
	var err HTTPErrorResponse
	return injectTracingHeaders(
		ctx, g.restyClient.R().
			SetContext(ctx).
			SetError(&err),
	)
}

func injectTracingHeaders(ctx context.Context, req *resty.Request) *resty.Request {
	// look for span in context, do nothing if span is not found
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return req
	}

	// look for tracer in context, use global tracer if not found
	tracer, ok := ctx.Value(tracerContextKey).(opentracing.Tracer)
	if !ok || tracer == nil {
		tracer = opentracing.GlobalTracer()
	}

	// inject tracing header into request
	err := tracer.Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		return req
	}

	return req
}

// GetRequestWithBearerAuthNoCache returns a JSON base request configured with an auth token and no-cache header.
func (g *GoZaya) GetRequestWithBearerAuthNoCache(ctx context.Context, token string) *resty.Request {
	return g.GetRequest(ctx).
		SetAuthToken(token).
		SetHeader("Content-Type", "application/json").
		SetHeader("Cache-Control", "no-cache")
}

// GetRequestWithBearerAuth returns a JSON base request configured with an auth token.
func (g *GoZaya) GetRequestWithBearerAuth(ctx context.Context, token string) *resty.Request {
	return g.GetRequest(ctx).
		SetAuthToken(token).
		SetHeader("Content-Type", "application/json")
}

func (g *GoZaya) GetRequestFormData(ctx context.Context, token string) *resty.Request {
	return g.GetRequest(ctx).
		SetAuthToken(token).
		SetHeader("Content-Type", "application/x-www-form-urlencoded")
}

func NewClient(basePath string, options ...func(*GoZaya)) *GoZaya {
	c := GoZaya{
		basePath:    strings.TrimRight(basePath, urlSeparator),
		restyClient: resty.New(),
	}

	c.Config.CreateLinkEndpoint = makeURL("api", "v1", "links")
	c.Config.GetLinkEndpoint = makeURL("api", "v1", "links")

	for _, option := range options {
		option(&c)
	}

	return &c
}

// RestyClient returns the internal resty g.
// This can be used to configure the g.
func (g *GoZaya) RestyClient() *resty.Client {
	return g.restyClient
}

// SetRestyClient overwrites the internal resty g.
func (g *GoZaya) SetRestyClient(restyClient *resty.Client) {
	g.restyClient = restyClient
	g.restyClient.SetTimeout(30 * time.Second)
}

func checkForError(resp *resty.Response, err error, errMessage string) error {
	if err != nil {
		return &APIError{
			Code:    0,
			Message: errors.Wrap(err, errMessage).Error(),
			Type:    ParseAPIErrType(err),
		}
	}

	if resp == nil {
		return &APIError{
			Message: "empty response",
			Type:    ParseAPIErrType(err),
		}
	}

	if resp.IsError() {
		var msg string

		// Parse the error message from the body if available
		if e, ok := resp.Error().(*HTTPErrorResponse); ok && e.NotEmpty() {
			msg = fmt.Sprintf("%s: %s", resp.Status(), e)
		} else if resp.Body() != nil {
			// If the body contains a message, include it
			bodyMsg := string(resp.Body())
			msg = fmt.Sprintf("%s: %s", resp.Status(), bodyMsg)
		} else {
			msg = resp.Status()
		}

		return &APIError{
			Code:    resp.StatusCode(),
			Message: msg,
			Type:    ParseAPIErrType(err),
		}
	}

	return nil
}

func (g *GoZaya) CreateLink(ctx context.Context, token string, link *GenerateLinkRequest) (*ResponseModel, error) {
	var result ResponseModel

	form := make(map[string]string)

	if link.Url != "" {
		form["url"] = link.Url
	}
	if link.Alias != "" {
		form["alias"] = link.Alias
	}
	if link.Password != "" {
		form["password"] = link.Password
	}
	if link.Disable != 0 {
		form["disable"] = strconv.Itoa(link.Disable)
	}
	if link.Public != 0 {
		form["public"] = strconv.Itoa(link.Public)
	}
	if link.Description != "" {
		form["description"] = link.Description
	}
	if link.ExpirationDate != "" {
		form["expiration_date"] = link.ExpirationDate
	}
	if link.ExpirationTime != "" {
		form["expiration_time"] = link.ExpirationTime
	}
	if link.ExpirationClicks != 0 {
		form["expiration_clicks"] = strconv.Itoa(link.ExpirationClicks)
	}
	if link.Domain != 0 {
		form["domain"] = strconv.Itoa(link.Domain)
	}
	if link.ExpirationUrl != "" {
		form["expiration_url"] = link.ExpirationUrl
	}

	resp, err := g.GetRequestFormData(ctx, token).
		SetFormData(form).
		Post(g.basePath + "/" + g.Config.CreateLinkEndpoint)

	if err := checkForError(resp, err, "failed to create link"); err != nil {
		return nil, err
	}

	return &result, nil
}

func (g *GoZaya) GetLink(ctx context.Context, token string, id string) (*ResponseModel, error) {
	var result ResponseModel

	resp, err := g.GetRequestWithBearerAuthNoCache(ctx, token).
		Get(g.basePath + "/" + g.Config.GetLinkEndpoint + "/" + id)

	if err := checkForError(resp, err, "failed to get link"); err != nil {
		return nil, err
	}

	return &result, nil
}
