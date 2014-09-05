package file

import (
	"encoding/json"
	"github.com/b3log/wide/conf"
	"github.com/b3log/wide/user"
	"github.com/b3log/wide/util"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func GetFiles(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{"succ": true}
	defer util.RetJSON(w, r, data)

	session, _ := user.Session.Get(r, "wide-session")

	username := session.Values["username"].(string)
	userRepos := conf.Wide.UserWorkspaces + string(os.PathSeparator) + username + string(os.PathSeparator) + "src"

	root := FileNode{"projects", userRepos, "d", []*FileNode{}}
	fileInfo, _ := os.Lstat(userRepos)

	walk(userRepos, fileInfo, &root)

	data["root"] = root
}

func GetFile(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{"succ": true}
	defer util.RetJSON(w, r, data)

	decoder := json.NewDecoder(r.Body)

	var args map[string]interface{}

	if err := decoder.Decode(&args); err != nil {
		glog.Error(err)
		data["succ"] = false

		return
	}

	path := args["path"].(string)

	buf, _ := ioutil.ReadFile(path)

	idx := strings.LastIndex(path, ".")
	extension := ""
	if 0 <= idx {
		extension = path[idx:]
	}

	// 通过文件扩展名判断是否是图片文件（图片在浏览器里新建 tab 打开）
	if isImg(extension) {
		data["mode"] = "img"

		path2 := strings.Replace(path, "\\", "/", -1)
		idx = strings.Index(path2, "/data/user_workspaces")
		data["path"] = path2[idx:]

		return
	}

	isBinary := false
	// 判断是否是其他二进制文件
	for _, b := range buf {
		if 0 == b { // 包含 0 字节就认为是二进制文件
			isBinary = true
		}
	}

	if isBinary {
		// 是二进制文件的话前端编辑器不打开
		data["succ"] = false
		data["msg"] = "Can't open a binary file :("
	} else {
		data["content"] = string(buf)
		data["mode"] = getEditorMode(extension)
	}
}

func SaveFile(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{"succ": true}
	defer util.RetJSON(w, r, data)

	decoder := json.NewDecoder(r.Body)

	var args map[string]interface{}

	if err := decoder.Decode(&args); err != nil {
		glog.Error(err)
		data["succ"] = false

		return
	}

	filePath := args["file"].(string)

	fout, err := os.Create(filePath)

	if nil != err {
		glog.Error(err)
		data["succ"] = false

		return
	}

	code := args["code"].(string)

	fout.WriteString(code)

	if err := fout.Close(); nil != err {
		glog.Error(err)
		data["succ"] = false

		return
	}
}

func NewFile(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{"succ": true}
	defer util.RetJSON(w, r, data)

	decoder := json.NewDecoder(r.Body)

	var args map[string]interface{}

	if err := decoder.Decode(&args); err != nil {
		glog.Error(err)
		data["succ"] = false

		return
	}

	path := args["path"].(string)
	fileType := args["fileType"].(string)

	if !createFile(path, fileType) {
		data["succ"] = false
	}
}

func RemoveFile(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{"succ": true}
	defer util.RetJSON(w, r, data)

	decoder := json.NewDecoder(r.Body)

	var args map[string]interface{}

	if err := decoder.Decode(&args); err != nil {
		glog.Error(err)
		data["succ"] = false

		return
	}

	path := args["path"].(string)

	if !removeFile(path) {
		data["succ"] = false
	}
}

type FileNode struct {
	Name      string      `json:"name"`
	Path      string      `json:"path"`
	Type      string      `json:"type"`
	FileNodes []*FileNode `json:"children"`
}

func walk(path string, info os.FileInfo, node *FileNode) {
	files := listFiles(path)

	for _, filename := range files {
		fpath := filepath.Join(path, filename)

		fio, _ := os.Lstat(fpath)

		child := FileNode{Name: filename, Path: fpath, FileNodes: []*FileNode{}}
		node.FileNodes = append(node.FileNodes, &child)

		if nil == fio {
			glog.Warningf("Path [%s] is nil", fpath)

			continue
		}

		if fio.IsDir() {
			child.Type = "d"

			walk(fpath, fio, &child)
		} else {
			child.Type = "f"
		}
	}

	return
}

func listFiles(dirname string) []string {
	f, _ := os.Open(dirname)

	names, _ := f.Readdirnames(-1)
	f.Close()

	sort.Strings(names)

	dirs := []string{}
	files := []string{}

	// 排序：目录靠前，文件靠后
	for _, name := range names {
		fio, _ := os.Lstat(filepath.Join(dirname, name))

		if fio.IsDir() {
			// 排除 .git 目录
			if ".git" == fio.Name() {
				continue
			}

			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}

	return append(dirs, files...)
}

func getEditorMode(filenameExtension string) string {
	switch filenameExtension {
	case ".go":
		return "go"
	case ".html":
		return "htmlmixed"
	case ".md":
		return "markdown"
	case ".js", ".json":
		return "javascript"
	case ".css":
		return "css"
	case ".xml":
		return "xml"
	case ".sh":
		return "shell"
	case ".sql":
		return "sql"
	default:
		return "text"
	}
}

func createFile(path, fileType string) bool {
	switch fileType {
	case "f":
		file, err := os.OpenFile(path, os.O_CREATE, 0664)
		if nil != err {
			glog.Info(err)

			return false
		}

		defer file.Close()

		glog.Infof("Created file [%s]", path)

		return true
	case "d":
		err := os.Mkdir(path, 0775)

		if nil != err {
			glog.Info(err)
		}

		glog.Infof("Created directory [%s]", path)

		return true
	default:
		glog.Infof("Unsupported file type [%s]", fileType)

		return false
	}
}

func removeFile(path string) bool {
	if err := os.RemoveAll(path); nil != err {
		glog.Errorf("Removes [%s] failed: [%s]", path, err.Error())

		return false
	}

	glog.Infof("Removed [%s]", path)

	return true
}

func isImg(extension string) bool {
	ext := strings.ToLower(extension)

	switch ext {
	case ".jpg", ".jpeg", ".bmp", ".gif", ".png", ".svg", ".ico":
		return true
	default:
		return false
	}
}
