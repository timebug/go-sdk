package upyun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type CommitTasksConfig struct {
	AppName   string
	Accept    string
	Source    string
	NotifyUrl string
	Tasks     []interface{}
}

type SyncTaskConfig struct {
	Param map[string]interface{}
}

type Reader interface {
	getKwargs() map[string]interface{}
}

func (up *UpYun) CommitTasks(config *CommitTasksConfig) (taskIds []string, err error) {
	b, err := json.Marshal(config.Tasks)
	if err != nil {
		return nil, err
	}

	kwargs := map[string]string{
		"app_name":   config.AppName,
		"tasks":      base64ToStr(b),
		"notify_url": config.NotifyUrl,

		// for naga
		"source": config.Source,
	}
	if config.Accept != "" {
		kwargs["accept"] = config.Accept
	}

	err = up.doProcessRequest("POST", "/pretreatment/", kwargs, &taskIds)
	return
}

func (up *UpYun) GetProgress(taskIds []string) (result map[string]int, err error) {
	kwargs := map[string]string{
		"task_ids": strings.Join(taskIds, ","),
	}
	v := map[string]map[string]int{}
	err = up.doProcessRequest("GET", "/status/", kwargs, &v)
	if err != nil {
		return
	}

	if r, ok := v["tasks"]; ok {
		return r, err
	}
	return nil, fmt.Errorf("no tasks")
}

func (up *UpYun) GetResult(taskIds []string) (result map[string]interface{}, err error) {
	kwargs := map[string]string{
		"task_ids": strings.Join(taskIds, ","),
	}
	v := map[string]map[string]interface{}{}
	err = up.doProcessRequest("GET", "/result/", kwargs, &v)
	if err != nil {
		return
	}

	if r, ok := v["tasks"]; ok {
		return r, err
	}
	return nil, fmt.Errorf("no tasks")
}

func (up *UpYun) doProcessRequest(method, uri string,
	kwargs map[string]string, v interface{}) error {
	if _, ok := kwargs["service"]; !ok {
		kwargs["service"] = up.Bucket
	}

	if method == "GET" {
		uri = addQueryToUri(uri, kwargs)
	}

	headers := make(map[string]string)
	headers["Date"] = makeRFC1123Date(time.Now())
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	if up.deprecated {
		headers["Authorization"] = up.MakeProcessAuth(kwargs)
	} else {
		headers["Authorization"] = up.MakeUnifiedAuth(&UnifiedAuthConfig{
			Method:  method,
			Uri:     uri,
			DateStr: headers["Date"],
		})
	}

	var resp *http.Response
	var err error
	endpoint := up.doGetEndpoint("p0.api.upyun.com")
	rawurl := fmt.Sprintf("http://%s%s", endpoint, uri)
	switch method {
	case "GET":
		resp, err = up.doHTTPRequest(method, rawurl, headers, nil)
	case "POST":
		payload := encodeQueryToPayload(kwargs)
		resp, err = up.doHTTPRequest(method, rawurl, headers, bytes.NewBufferString(payload))
	default:
		return fmt.Errorf("Unknown method")
	}

	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%d %s", resp.StatusCode, string(b))
	}

	return json.Unmarshal(b, v)
}

//同步任务提交
func (up *UpYun) CommitSyncTasks(commitTask Reader, taskUri string) (result map[string]interface{}, err error) {
	kwargs := make(map[string]interface{})
	var payload string
	var uri string

	kwargs = commitTask.getKwargs()
	if _, exist := kwargs["service"]; !exist {
		kwargs["service"] = up.Bucket
	}
	uri = fmt.Sprintf("/%v%v", up.Bucket, taskUri)

	body, err := json.Marshal(kwargs)
	if err != nil {
		return nil, fmt.Errorf("can't encode the json")
	}
	payload = string(body)

	return up.doSyncProcessRequest("POST", uri, payload)
}

func (up *UpYun) doSyncProcessRequest(method, uri string, payload string) (map[string]interface{}, error) {
	headers := make(map[string]string)
	headers["Date"] = makeRFC1123Date(time.Now())
	headers["Content-Type"] = "application/json"
	headers["Content-MD5"] = md5Str(payload)
	headers["Authorization"] = up.MakeUnifiedAuth(&UnifiedAuthConfig{
		Method:     method,
		Uri:        uri,
		DateStr:    headers["Date"],
		ContentMD5: headers["Content-MD5"],
	})

	var resp *http.Response
	var err error
	endpoint := up.doGetEndpoint("p1.api.upyun.com")
	rawurl := fmt.Sprintf("http://%s%s", endpoint, uri)
	switch method {
	case "POST":
		resp, err = up.doHTTPRequest(method, rawurl, headers, strings.NewReader(payload))
	default:
		return nil, fmt.Errorf("Unknown method")
	}
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%d %s", resp.StatusCode, string(b))
	}

	var v map[string]interface{}
	err = json.Unmarshal(b, &v)
	if err != nil {
		fmt.Println("can't unmarshal the data", string(b))
	}
	return v, err
}

func (config *SyncTaskConfig) getKwargs() map[string]interface{} {
	return config.Param
}
