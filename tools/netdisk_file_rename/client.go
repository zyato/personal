package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL = "https://pan.baidu.com"
	fileAPIPath       = "/rest/2.0/xpan/file"
	listPageSize      = 1000
	maxResponseBytes  = 4 * 1024 * 1024
	maxListEntries    = 100000
)

type Client struct {
	accessToken string
	baseURL     string
	httpClient  *http.Client
	limiter     *rateLimiter
}

func NewClient(accessToken string, limiter *rateLimiter, timeout time.Duration) *Client {
	return newClientWithBaseURL(accessToken, limiter, timeout, defaultAPIBaseURL)
}

func newClientWithBaseURL(accessToken string, limiter *rateLimiter, timeout time.Duration, baseURL string) *Client {
	return &Client{
		accessToken: accessToken,
		baseURL:     strings.TrimRight(baseURL, "/"),
		limiter:     limiter,
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				// Do not forward a URL containing access_token to a redirect target.
				return http.ErrUseLastResponse
			},
		},
	}
}

type apiEnvelope struct {
	Errno     *int            `json:"errno"`
	ErrMsg    string          `json:"errmsg"`
	ShowMsg   string          `json:"show_msg"`
	RequestID json.RawMessage `json:"request_id"`
}

type listResponse struct {
	List []FileEntry `json:"list"`
}

type renameResponse struct {
	TaskID uint64 `json:"taskid"`
	Info   []struct {
		Errno int    `json:"errno"`
		Path  string `json:"path"`
	} `json:"info"`
}

type APIError struct {
	Operation string
	Errno     int
	RequestID string
}

func (err *APIError) Error() string {
	message := apiErrorMessage(err.Errno)
	if err.RequestID != "" {
		return fmt.Sprintf("%s失败: errno=%d（%s），request_id=%s", err.Operation, err.Errno, message, err.RequestID)
	}
	return fmt.Sprintf("%s失败: errno=%d（%s）", err.Operation, err.Errno, message)
}

type networkError struct {
	message string
}

func (err *networkError) Error() string { return err.message }

type preRequestError struct {
	err error
}

func (err *preRequestError) Error() string { return err.err.Error() }
func (err *preRequestError) Unwrap() error { return err.err }

type httpStatusError struct {
	operation string
	status    int
}

func (err *httpStatusError) Error() string {
	if err.status == http.StatusTooManyRequests {
		return fmt.Sprintf("%s失败: HTTP 429，已触发限流；请停止调用并核对应用配额", err.operation)
	}
	return fmt.Sprintf("%s失败: HTTP %d", err.operation, err.status)
}

func (client *Client) ListFiles(ctx context.Context, dir string) ([]FileEntry, error) {
	entries := make([]FileEntry, 0)
	for start := 0; ; {
		query := url.Values{
			"method":    {"list"},
			"dir":       {dir},
			"order":     {"name"},
			"desc":      {"0"},
			"start":     {strconv.Itoa(start)},
			"limit":     {strconv.Itoa(listPageSize)},
			"web":       {"0"},
			"folder":    {"0"},
			"showempty": {"0"},
		}
		var response listResponse
		_, err := client.do(ctx, http.MethodGet, "查询文件列表", query, nil, &response)
		if err != nil {
			return nil, err
		}
		entries = append(entries, response.List...)
		if len(response.List) < listPageSize {
			return entries, nil
		}
		if len(entries) > maxListEntries {
			return nil, fmt.Errorf("目录条目超过 %d，为避免长时间占用配额已停止", maxListEntries)
		}
		start += len(response.List)
	}
}

// RenameFile deliberately sends one file with async=0 and ondup=fail. Mutating
// requests are never automatically retried because a timeout has an unknown result.
func (client *Client) RenameFile(ctx context.Context, sourcePath, newName string) (string, error) {
	fileList, err := json.Marshal([]struct {
		Path    string `json:"path"`
		NewName string `json:"newname"`
	}{{Path: sourcePath, NewName: newName}})
	if err != nil {
		return "", fmt.Errorf("编码重命名参数失败: %w", err)
	}
	query := url.Values{
		"method": {"filemanager"},
		"opera":  {"rename"},
	}
	form := url.Values{
		"async":    {"0"},
		"ondup":    {"fail"},
		"filelist": {string(fileList)},
	}
	var response renameResponse
	requestID, err := client.do(ctx, http.MethodPost, "重命名文件", query, form, &response)
	if err != nil {
		return requestID, err
	}
	if response.TaskID != 0 {
		return requestID, fmt.Errorf("服务端返回异步任务 taskid=%d，结果待人工核对；工具不会自动重试或轮询", response.TaskID)
	}
	if len(response.Info) != 1 {
		return requestID, fmt.Errorf("重命名响应缺少明确的单文件结果，请人工核对后再操作")
	}
	result := response.Info[0]
	if result.Errno != 0 {
		return requestID, &APIError{Operation: "重命名文件", Errno: result.Errno, RequestID: requestID}
	}
	if result.Path != "" && result.Path != sourcePath {
		return requestID, fmt.Errorf("重命名响应路径与请求不一致，请人工核对")
	}
	return requestID, nil
}

// do retries only idempotent GET requests on network errors and HTTP 5xx.
// HTTP 429 and all business errors stop immediately.
func (client *Client) do(ctx context.Context, method, operation string, query, form url.Values, output any) (string, error) {
	query.Set("access_token", client.accessToken)
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		requestID, err := client.doOnce(ctx, method, operation, query, form, output)
		if err == nil {
			return requestID, nil
		}
		if method != http.MethodGet || attempt == maxAttempts-1 || !retryable(err) || ctx.Err() != nil {
			return requestID, err
		}
		if waitErr := waitForBackoff(ctx, time.Duration(5<<attempt)*time.Second); waitErr != nil {
			return requestID, waitErr
		}
	}
	return "", nil
}

func (client *Client) doOnce(ctx context.Context, method, operation string, query, form url.Values, output any) (string, error) {
	if err := client.limiter.wait(ctx); err != nil {
		return "", &preRequestError{err: fmt.Errorf("等待限速窗口失败: %w", err)}
	}
	requestURL := client.baseURL + fileAPIPath + "?" + query.Encode()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return "", &preRequestError{err: fmt.Errorf("创建请求失败: %s", redactSecret(err.Error(), client.accessToken))}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "pan.baidu.com")
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", &networkError{message: fmt.Sprintf("%s网络请求失败: %s", operation, redactSecret(err.Error(), client.accessToken))}
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("读取%s响应失败: %w", operation, err)
	}
	if len(raw) > maxResponseBytes {
		return "", fmt.Errorf("%s响应超过大小限制", operation)
	}

	var envelope apiEnvelope
	jsonErr := json.Unmarshal(raw, &envelope)
	requestID := rawRequestID(envelope.RequestID)
	if jsonErr == nil && envelope.Errno != nil && *envelope.Errno != 0 {
		return requestID, &APIError{Operation: operation, Errno: *envelope.Errno, RequestID: requestID}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return requestID, &httpStatusError{operation: operation, status: response.StatusCode}
	}
	if jsonErr != nil {
		return requestID, fmt.Errorf("%s返回了无效 JSON", operation)
	}
	if envelope.Errno == nil {
		return requestID, fmt.Errorf("%s响应缺少 errno", operation)
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return requestID, fmt.Errorf("解析%s响应失败: %w", operation, err)
	}
	return requestID, nil
}

func retryable(err error) bool {
	var networkErr *networkError
	if errors.As(err, &networkErr) {
		return true
	}
	var statusErr *httpStatusError
	return errors.As(err, &statusErr) && statusErr.status >= 500
}

// renameOutcomeUnknown distinguishes a definite server-side rejection from an
// ambiguous transport/response outcome that must be checked manually.
func renameOutcomeUnknown(err error) bool {
	if err == nil {
		return false
	}
	var beforeRequest *preRequestError
	if errors.As(err, &beforeRequest) {
		return false
	}
	var networkErr *networkError
	if errors.As(err, &networkErr) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return false
	}
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status >= 500
	}
	// Any other error from RenameFile occurs after the request was sent (for
	// example, a response-body timeout or an incomplete success response).
	return true
}

func waitForBackoff(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func rawRequestID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
}

func redactSecret(message, secret string) string {
	if secret == "" {
		return message
	}
	for _, candidate := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret)} {
		if candidate != "" {
			message = strings.ReplaceAll(message, candidate, "<ACCESS_TOKEN>")
		}
	}
	return message
}

func apiErrorMessage(errno int) string {
	messages := map[int]string{
		-3:    "文件不存在，请重新预演核对目录",
		-6:    "身份验证失败，请重新完成 OAuth 授权",
		-7:    "文件或目录名错误，或无权访问",
		-8:    "目标文件或目录已存在",
		-9:    "文件或目录不存在，请重新预演核对目录",
		2:     "参数错误，请核对官方接口文档",
		111:   "存在其他异步任务，请停止并稍后人工核对",
		20011: "未审核应用仅允许前 10 个授权用户测试",
		20012: "访问次数或频率已达上限，请勿立即重试并检查应用配额",
		20013: "当前应用没有该接口权限",
		20016: "access_token 已过期，请重新授权或续期",
		20017: "access_token 无效，请重新授权",
		20020: "路径不在应用授权范围内",
		20022: "路径参数格式不正确",
		20023: "缺少必需路径参数",
		31023: "参数错误，请核对官方接口文档",
		31024: "没有访问权限",
		31034: "命中接口频控，请勿立即重试",
		31045: "access_token 验证未通过",
		31061: "目标文件已存在",
		31062: "文件名无效",
		31066: "文件不存在",
	}
	if message, ok := messages[errno]; ok {
		return message
	}
	return "请依据百度网盘开放平台错误码文档排查"
}
