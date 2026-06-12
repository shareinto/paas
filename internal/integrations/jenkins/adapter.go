package jenkins

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/shareinto/paas/internal/modules/build"
	"github.com/shareinto/paas/internal/shared"
)

type Adapter struct{ client *Client }

func NewAdapter(client *Client) *Adapter { return &Adapter{client: client} }

func (a *Adapter) EnsureJob(ctx context.Context, spec build.BuildJobSpec) error {
	segments := jobSegments(spec.JobName)
	if len(segments) == 0 {
		return shared.NewError(shared.CodeInvalidArgument, "jenkins job name is required")
	}
	parent := ""
	for _, folder := range segments[:len(segments)-1] {
		if err := a.ensureFolder(ctx, parent, folder); err != nil {
			return err
		}
		if parent == "" {
			parent = folder
		} else {
			parent += "/" + folder
		}
	}
	xmlContent := strings.TrimSpace(spec.TemplateXML)
	if xmlContent == "" {
		xmlContent = "<project><description>" + spec.TemplateID + "</description></project>"
	}
	createPath := jenkinsJobPath(parent) + "/createItem?name=" + url.QueryEscape(segments[len(segments)-1])
	if parent == "" {
		createPath = "/createItem?name=" + url.QueryEscape(segments[len(segments)-1])
	}
	req, err := a.request(ctx, http.MethodPost, createPath, strings.NewReader(xmlContent))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")
	if err := a.do(req, nil); err != nil {
		if shared.CodeOf(err) == shared.CodeConflict {
			return a.updateJobConfig(ctx, spec.JobName, xmlContent)
		}
		return err
	}
	return nil
}

func (a *Adapter) updateJobConfig(ctx context.Context, jobName string, xmlContent string) error {
	req, err := a.request(ctx, http.MethodPost, jenkinsJobPath(jobName)+"/config.xml", strings.NewReader(xmlContent))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")
	return a.do(req, nil)
}

func (a *Adapter) DeleteJob(ctx context.Context, jobName string) error {
	segments := jobSegments(jobName)
	if len(segments) == 0 {
		return shared.NewError(shared.CodeInvalidArgument, "jenkins job name is required")
	}
	req, err := a.request(ctx, http.MethodPost, jenkinsJobPath(jobName)+"/doDelete", nil)
	if err != nil {
		return err
	}
	if err := a.do(req, nil); err != nil {
		if shared.CodeOf(err) == shared.CodeNotFound {
			return nil
		}
		return err
	}
	return nil
}

func (a *Adapter) ensureFolder(ctx context.Context, parent string, name string) error {
	createPath := jenkinsJobPath(parent) + "/createItem?name=" + url.QueryEscape(name)
	if parent == "" {
		createPath = "/createItem?name=" + url.QueryEscape(name)
	}
	req, err := a.request(ctx, http.MethodPost, createPath, strings.NewReader("<com.cloudbees.hudson.plugins.folder.Folder/>"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")
	if err := a.do(req, nil); err != nil && shared.CodeOf(err) != shared.CodeConflict {
		return err
	}
	return nil
}

func (a *Adapter) TriggerBuild(ctx context.Context, jobName string, parameters map[string]string) (build.BuildQueueItem, error) {
	if len(parameters) == 0 {
		req, err := a.request(ctx, http.MethodPost, jenkinsJobPath(jobName)+"/build", nil)
		if err != nil {
			return build.BuildQueueItem{}, err
		}
		resp, err := a.client.do(req, nil)
		if err != nil {
			return build.BuildQueueItem{}, err
		}
		defer resp.Body.Close()
		return build.BuildQueueItem{QueueID: resp.Header.Get("Location")}, nil
	}
	form := url.Values{}
	for key, value := range parameters {
		form.Set(key, value)
	}
	req, err := a.request(ctx, http.MethodPost, jenkinsJobPath(jobName)+"/buildWithParameters", strings.NewReader(form.Encode()))
	if err != nil {
		return build.BuildQueueItem{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.do(req, nil)
	if err != nil {
		return build.BuildQueueItem{}, err
	}
	defer resp.Body.Close()
	location := resp.Header.Get("Location")
	return build.BuildQueueItem{QueueID: location}, nil
}

func (a *Adapter) GetQueueItem(ctx context.Context, queueID string) (build.BuildQueueItem, error) {
	req, err := a.request(ctx, http.MethodGet, queueID+"/api/json", nil)
	if err != nil {
		return build.BuildQueueItem{}, err
	}
	var out struct {
		Cancelled  bool `json:"cancelled"`
		Executable *struct {
			Number int64 `json:"number"`
		} `json:"executable"`
	}
	if err := a.do(req, &out); err != nil {
		return build.BuildQueueItem{}, err
	}
	item := build.BuildQueueItem{QueueID: queueID, Canceled: out.Cancelled}
	if out.Executable != nil {
		item.Started = true
		item.BuildNumber = out.Executable.Number
	}
	return item, nil
}

func (a *Adapter) GetBuildStatus(ctx context.Context, jobName string, buildNumber int64) (build.BuildStatus, error) {
	req, err := a.request(ctx, http.MethodGet, jenkinsJobPath(jobName)+"/"+strconv.FormatInt(buildNumber, 10)+"/api/json?tree=number,building,result", nil)
	if err != nil {
		return build.BuildStatus{}, err
	}
	var out struct {
		Number   int64  `json:"number"`
		Building bool   `json:"building"`
		Result   string `json:"result"`
	}
	if err := a.do(req, &out); err != nil {
		return build.BuildStatus{}, err
	}
	status := build.BuildStatus{BuildNumber: out.Number, Building: out.Building}
	if status.BuildNumber == 0 {
		status.BuildNumber = buildNumber
	}
	status.Status = mapBuildResult(out.Result, out.Building)
	return status, nil
}

func (a *Adapter) ProgressiveText(ctx context.Context, jobName string, buildNumber int64, offset int64) (build.ProgressiveText, error) {
	req, err := a.request(ctx, http.MethodGet, jenkinsJobPath(jobName)+"/"+strconv.FormatInt(buildNumber, 10)+"/logText/progressiveText?start="+strconv.FormatInt(offset, 10), nil)
	if err != nil {
		return build.ProgressiveText{}, err
	}
	resp, err := a.client.do(req, nil)
	if err != nil {
		return build.ProgressiveText{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	next, _ := strconv.ParseInt(resp.Header.Get("X-Text-Size"), 10, 64)
	return build.ProgressiveText{Text: redact(string(data)), NextOffset: next, MoreData: resp.Header.Get("X-More-Data") == "true"}, nil
}

func (a *Adapter) CancelBuild(ctx context.Context, jobName string, buildNumber int64) error {
	req, err := a.request(ctx, http.MethodPost, jenkinsJobPath(jobName)+"/"+strconv.FormatInt(buildNumber, 10)+"/stop", nil)
	if err != nil {
		return err
	}
	return a.do(req, nil)
}

func (a *Adapter) CancelQueueItem(ctx context.Context, queueID string) error {
	id, err := queueItemID(queueID)
	if err != nil {
		return err
	}
	req, err := a.request(ctx, http.MethodPost, "/queue/cancelItem?id="+url.QueryEscape(id), nil)
	if err != nil {
		return err
	}
	return a.do(req, nil)
}

func queueItemID(queueID string) (string, error) {
	value := strings.TrimSpace(queueID)
	if value == "" {
		return "", shared.NewError(shared.CodeInvalidArgument, "jenkins queue id is required")
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", err
		}
		value = parsed.Path
	}
	value = strings.Trim(value, "/")
	parts := strings.Split(value, "/")
	id := strings.TrimSpace(parts[len(parts)-1])
	if id == "" {
		return "", shared.NewError(shared.CodeInvalidArgument, "jenkins queue id is required")
	}
	return id, nil
}

func jobSegments(jobName string) []string {
	parts := strings.Split(strings.Trim(jobName, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func jenkinsJobPath(jobName string) string {
	segments := jobSegments(jobName)
	if len(segments) == 0 {
		return ""
	}
	var b strings.Builder
	for _, segment := range segments {
		b.WriteString("/job/")
		b.WriteString(url.PathEscape(segment))
	}
	return b.String()
}

func mapBuildResult(result string, building bool) build.BuildRunStatus {
	if building {
		return build.BuildRunRunning
	}
	switch strings.ToUpper(strings.TrimSpace(result)) {
	case "SUCCESS":
		return build.BuildRunSucceeded
	case "FAILURE":
		return build.BuildRunFailed
	case "ABORTED":
		return build.BuildRunAborted
	case "UNSTABLE":
		return build.BuildRunUnstable
	default:
		return ""
	}
}

func (a *Adapter) request(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	target := path
	if !strings.HasPrefix(path, "http") {
		target = a.client.baseURL + path
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	if a.client.username != "" || a.client.token != "" {
		req.SetBasicAuth(a.client.username, a.client.token)
	}
	return req, nil
}

func (a *Adapter) do(req *http.Request, target any) error {
	resp, err := a.client.do(req, target)
	if resp != nil {
		_ = resp.Body.Close()
	}
	return err
}

func redact(value string) string {
	for _, marker := range []string{"token=", "password=", "secret="} {
		idx := strings.Index(strings.ToLower(value), marker)
		if idx >= 0 {
			end := strings.IndexAny(value[idx+len(marker):], " \n\r\t")
			if end < 0 {
				return value[:idx+len(marker)] + "[REDACTED]"
			}
			end += idx + len(marker)
			value = value[:idx+len(marker)] + "[REDACTED]" + value[end:]
		}
	}
	return value
}
