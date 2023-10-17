package cmd

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	//go:embed assets/*
	fileSystem embed.FS
)

// Md5Sum 计算文件的md5值
func Md5Sum(path string) (string, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := md5.New()
	bf := make([]byte, 512*1024)
	for {
		n, err := file.Read(bf)
		if err != nil {
			if err != io.EOF {
				return "", err
			}
			return hex.EncodeToString(hash.Sum(nil)), nil
		}
		if n > 0 {
			_, err := hash.Write(bf[0:n])
			if err != nil {
				return "", err
			}
		}
	}
}

// ConvertPropertiesToMap 将Properties转换成map
func ConvertPropertiesToMap(file *zip.File) (map[string]string, error) {
	result := make(map[string]string)
	handler, err := file.Open()
	if err != nil {
		return result, err
	}
	defer handler.Close()
	reader := bufio.NewReader(handler)
	for line, err := reader.ReadString('\n'); err == nil; line, err = reader.ReadString('\n') {
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		splitN := strings.SplitN(line, "=", 2)
		result[splitN[0]] = splitN[1]
	}
	return result, nil
}

// ZipFileToReader 将zip file转换成 zip.reader
func ZipFileToReader(file *zip.File) (*zip.Reader, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	bts, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return zip.NewReader(bytes.NewReader(bts), int64(len(bts)))
}

// VersionCompare 通用版本比较函数
func VersionCompare(v1 string, v2 string) int {
	splitFunc := func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	}
	v1Ar := strings.FieldsFunc(v1, splitFunc)
	v2Ar := strings.FieldsFunc(v2, splitFunc)
	for i := 0; i < len(v1Ar) && i < len(v2Ar); i++ {
		v1a := v1Ar[i]
		v2a := v2Ar[i]
		if strings.Compare(v1a, v2a) == 0 {
			continue
		}
		if v1n, err := strconv.Atoi(v1Ar[i]); err == nil {
			if v2n, err := strconv.Atoi(v2Ar[i]); err == nil {
				if v1n != v2n {
					return v1n - v2n
				}
			}
		}
		return strings.Compare(v1a, v2a)
	}
	return len(v1Ar) - len(v2Ar)
}

// 防止golang未使用变量导致编译不通过
func Unused(obj interface{}) {

}

func CopyHeader(from http.Header, to http.Header, excludes ...string) {
out:
	for k, vv := range from {
		for _, exclude := range excludes {
			if strings.EqualFold(exclude, k) {
				continue out
			}
		}
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		to[k] = vv2
	}
}

// 判断包含
func ContainsFold(dest string, ranges ...string) bool {
	for _, e := range ranges {
		if strings.EqualFold(e, dest) {
			return true
		}
	}
	return false
}

// 获取本地ip地址
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() {
			if ip.IP.To4() != nil {
				return ip.IP.String()
			}
		}
	}
	return ""
}

type Pair struct {
	l int64
	r int64
}

func StepRange(lid, rid, step int64) []Pair {
	pairs := make([]Pair, 0, 16)
	if lid >= rid || step <= 0 {
		return pairs
	}

	for i := lid; i < rid; i = i + step {
		h := rid
		if i+step < rid {
			h = i + step
		}
		pair := Pair{
			l: i,
			r: h,
		}
		pairs = append(pairs, pair)

	}
	return pairs
}
