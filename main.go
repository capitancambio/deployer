package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

var token *string = flag.String("token", "", "github authorasation token")
var files *string = flag.String("files", "", "a comma separated list of files to deploy")
var tag *string = flag.String("tag", "SNAPSHOT", "The tag")
var desc *string = flag.String("desc", "current binaries", "desciption of the deployment")
var repo *string = flag.String("repo", "", "repository")
var user *string = flag.String("user", "", "user")

var (
	BaseURL   = "https://api.github.com"
	UploadUrl = "https://uploads.github.com"
)

//{
//"tag_name": "SNAPSHOT",
//"target_commitish": "master",
//"name": "SNAPSHOT",
//"body": "snapshot of the current state",
//"draft": false,
//"prerelease": true
//}

type Api struct {
	Token string
	Repo  string
	User  string
}
type Release struct {
	Tag        string `json:"tag_name"`
	Target     string `json:"target_commitish"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Id         int    `json:"id"`
}

func (a Api) GetReleaseByTag(tag string) (r Release, err error) {

	rs, err := a.ListReleases()
	if err != nil {
		fmt.Printf("err %+v\n", err)
		return
	}
	for _, rel := range rs {
		if rel.Tag == tag {
			return rel, nil
		}
	}

	fmt.Printf(" rs %+v\n", rs)
	return
}

func (a Api) BuildUrl() string {
	return fmt.Sprintf("%s/repos/%s/%s", BaseURL, a.User, a.Repo)
}
func (a Api) BuildUploadUrl() string {
	return fmt.Sprintf("%s/repos/%s/%s", UploadUrl, a.User, a.Repo)
}
func (a Api) Request(method, url string, reader io.Reader) (r *http.Request, err error) {
	r, err = http.NewRequest(method, url, reader)
	if err != nil {
		return
	}
	r.Header.Add("Authorization", " token "+a.Token)
	return
}
func (a Api) CreateRelease(r Release) (resp Release, err error) {
	w := bytes.NewBuffer([]byte{})
	if err = json.NewEncoder(w).Encode(r); err != nil {
		return
	}
	fmt.Println("Creating new release")
	url := fmt.Sprintf("%s%s", a.BuildUrl(), "/releases")
	req, err := a.Request("POST", url, w)

	req.Header.Add("Content-type", "application/json")
	res, err := (&http.Client{}).Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode != http.StatusCreated {
		return resp, fmt.Errorf("error create status status %v", res.Status)
	}
	err = json.NewDecoder(res.Body).Decode(&resp)
	return

}
func (a Api) ListReleases() (rs []Release, err error) {
	url := fmt.Sprintf("%s%s", a.BuildUrl(), "/releases")
	fmt.Printf("url %+v\n", url)
	req, err := a.Request("GET", url, nil)

	res, err := (&http.Client{}).Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode != http.StatusOK {
		fmt.Printf("res %+v\n", res)
		return rs, fmt.Errorf("Listing releases status %v", res.Status)
	}

	err = json.NewDecoder(res.Body).Decode(&rs)
	if err != nil {
		return
	}
	return
}

type Asset struct {
	Url string `json:"url"`

	Name string `json:"name"`

	Id    int    `json:"id"`
	State string `json:"state"`
}

func (a Api) ListAssets(r Release) (as []Asset, err error) {
	fmt.Println("Listing assets")
	url := fmt.Sprintf("%s%s/%v%s", a.BuildUrl(), "/releases", r.Id, "/assets")
	fmt.Printf("url %+v\n", url)
	req, err := a.Request("GET", url, nil)

	res, err := (&http.Client{}).Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode != http.StatusOK {
		return as, fmt.Errorf("Listing releases status %v", res.Status)
	}

	err = json.NewDecoder(res.Body).Decode(&as)
	return
}

func (a Api) CleanAssets(r Release) error {
	fmt.Println("Cleaning assets")
	asses, err := a.ListAssets(r)
	if err != nil {
		return err
	}
	for _, ass := range asses {
		fmt.Print("Deleting asset ", ass.Name, ass.Url)
		req, err := a.Request("DELETE", ass.Url, nil)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		res, err := (&http.Client{}).Do(req)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		if res.StatusCode != http.StatusOK {
			fmt.Printf("res %+v\n", res.Status)
		}

	}
	return nil
}
func (a Api) UploadAsset(r Release, file string) error {
	fmt.Println("Uploading asset", file)
	f, err := os.Open(file)

	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(f)
	url := fmt.Sprintf("%s/releases/%v/assets?name=%s", a.BuildUploadUrl(), r.Id, file)
	req, err := a.Request("POST", url, bytes.NewBuffer(data))
	req.Header.Add("Content-type", "application/octet-stream")
	if err != nil {
		return err
	}

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusCreated {
		result := map[string]interface{}{}
		err2 := json.NewDecoder(res.Body).Decode(&result)
		if err2 != nil {
			return err2

		}
		fmt.Printf("Result %+v\n", result)
		return err
	}
	return nil

}

func main() {

	flag.Parse()
	if *token == "" {
		fmt.Println("No token was provided")
		os.Exit(-1)
	}
	if *files == "" {
		fmt.Println("No files were specified")
		os.Exit(-1)
	}
	if *repo == "" {
		fmt.Println("No repo was provided")
		os.Exit(-1)
	}
	if *user == "" {
		fmt.Println("No user was propvided")
		os.Exit(-1)
	}
	api := Api{
		Token: *token,
		User:  *user,
		Repo:  *repo,
	}
	fileList := strings.Split(*files, ",")
	fmt.Printf("fileList %+v\n", fileList)
	r, err := api.GetReleaseByTag(*tag)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}
	if r.Name == "" {
		r, err = api.CreateRelease(Release{
			Tag:        *tag,
			Name:       *tag,
			Body:       *desc,
			Prerelease: true,
			Target:     "master",
		})
		if err != nil {
			os.Exit(-1)
			fmt.Println(err.Error())
		}
	}
	err = api.CleanAssets(r)
	if err != nil {
		fmt.Println("Error ", err.Error())
		os.Exit(-1)
	}

	dones := make(chan string)
	for _, file := range fileList {
		f := file
		go func() {
			err := api.UploadAsset(r, f)
			if err != nil {
				fmt.Println(f, ":   ", err.Error())

			}
			dones <- f
		}()
	}
	for i := 0; i < len(fileList); i++ {
		f := <-dones
		fmt.Printf("Done %+v\n", f)
	}

}
