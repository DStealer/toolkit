package cmd

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"errors"
	"github.com/magiconair/properties"
	"io"
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

// ReadProperties 将Properties转换成map
func ReadProperties(file *zip.File) (*properties.Properties, error) {
	handler, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer handler.Close()
	bts, err := io.ReadAll(handler)
	if err != nil {
		return nil, err
	}
	return properties.Load(bts, properties.ISO_8859_1)
}

// ConvertZipFileToReader 将zip file转换成 zip.reader
func ConvertZipFileToReader(file *zip.File) (*zip.Reader, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	bts, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return zip.NewReader(bytes.NewReader(bts), int64(len(bts)))
}

// Md5SumZipFile 计算给定zip.File的MD5哈希值
//
// 参数：
//
//	file *zip.File: 需要计算MD5哈希值的zip.File对象
//
// 返回值：
//
//	string: zip.File的MD5哈希值的十六进制字符串表示
//	error: 如果计算哈希值失败，返回错误
func Md5SumZipFile(file *zip.File) (string, error) {
	reader, err := file.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	hash := md5.New()
	bf := make([]byte, 512*1024)
	for {
		n, err := reader.Read(bf)
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

// Unused 防止golang未使用变量导致编译不通过
func Unused(obj interface{}) {

}

// CopyHeader 复制 from 中的 HTTP Header 到 to 中，排除 excludes 中指定的 Header。
//
// 参数：
// from: 源 HTTP Header。
// to: 目标 HTTP Header。
// excludes: 需要排除的 Header 名称列表。
//
// 返回值：
// 无返回值。
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

// ContainsFold 判断包含
func ContainsFold(dest string, ranges ...string) bool {
	for _, e := range ranges {
		if strings.EqualFold(e, dest) {
			return true
		}
	}
	return false
}

// GetLocalIP 函数返回本机的IPv4地址字符串
// 如果获取地址失败，则返回字符串"127.0.0.1"
// 如果本机没有IPv4地址，则返回空字符串""
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

type PairGenerator interface {
	Next() (bool, int64, int64)
	Pre() (bool, int64, int64)
	NextBoundary() (bool, int64, int64)
	PreBoundary() (bool, int64, int64)
}

type defaultPairGenerator struct {
	lindex int64
	left   int64
	rindex int64
	right  int64
	step   int64
}

// Next 方法返回布尔值、左边界和右边界，表示下一对数的生成范围
// 如果左边界大于等于右边界，则返回false、0和0
// 每次调用该方法后，左边界增加pg.step
// 如果计算出的右边界大于当前的右边界，则右边界不变
func (pg *defaultPairGenerator) Next() (bool, int64, int64) {
	if pg.lindex >= pg.rindex {
		return false, 0, 0
	}
	lindex := pg.lindex

	rindex := pg.lindex + pg.step

	if rindex > pg.rindex {
		rindex = pg.rindex
	}
	pg.lindex = rindex

	return true, lindex, rindex
}

// Pre 方法返回布尔值、左边界和右边界，表示下一对数的生成范围
// 如果左边界大于等于右边界，则返回false、0和0
// 每次调用该方法后，右边界变为左边界减去步长pg.step
// 如果计算出的左边界小于当前的左边界，则左边界不变
func (pg *defaultPairGenerator) Pre() (bool, int64, int64) {
	if pg.lindex >= pg.rindex {
		return false, 0, 0
	}
	rindex := pg.rindex

	lindex := pg.rindex - pg.step

	if lindex < pg.lindex {
		lindex = pg.lindex
	}
	pg.rindex = lindex

	return true, lindex, rindex
}

// NextBoundary 方法返回布尔值、左边界和右边界，表示下一对数的生成范围
// 如果左边界大于右边界，则返回false、0和0
// 每次调用该方法后，左边界增加pg.step，右边界不变
func (pg *defaultPairGenerator) NextBoundary() (bool, int64, int64) {
	if pg.lindex > pg.rindex {
		return false, 0, 0
	}
	lindex := pg.lindex

	rindex := pg.lindex + pg.step - 1

	if rindex > pg.rindex {
		rindex = pg.rindex
	}

	pg.lindex = rindex + 1

	return true, lindex, rindex
}

// PreBoundary 方法返回布尔值、左边界和右边界，表示下一对数的生成范围
// 如果左边界大于右边界，则返回false、0和0
// 如果左边界小于计算出的左边界，则更新左边界为计算出的左边界
// 每次调用该方法后，右边界减1
func (pg *defaultPairGenerator) PreBoundary() (bool, int64, int64) {
	if pg.lindex > pg.rindex {
		return false, 0, 0
	}
	rindex := pg.rindex

	lindex := pg.rindex - pg.step + 1

	if lindex < pg.lindex {
		lindex = pg.lindex
	}

	pg.rindex = rindex - 1

	return true, lindex, rindex
}

// NewPairGenerator 函数用于生成一个PairGenerator实例，并返回PairGenerator和错误信息
// left为生成范围的左边界
// right为生成范围的右边界
// step为生成范围的步长
// 返回值为PairGenerator和错误信息，如果left > right或step < 1，则返回nil和错误信息
func NewPairGenerator(left int64, right int64, step int64) (PairGenerator, error) {
	if left > right {
		return nil, errors.New("边界错误")
	}
	if step < 1 {
		return nil, errors.New("步长错误")
	}
	return &defaultPairGenerator{lindex: left, left: left, rindex: right, right: right, step: step}, nil
}

// If 函数接收一个bool类型的condition参数和两个interface{}类型的参数trueVal和falseVal
// 如果condition为true，则返回trueVal；否则返回falseVal
// 返回值为interface{}类型
func If(condition bool, trueVal, falseVal interface{}) interface{} {
	if condition {
		return trueVal
	}
	return falseVal
}

// ParsePermalinks 函数用于解析jenkins Permalinks 文件，返回一个map[string]string类型的变量
func ParsePermalinks(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var buildInfo = make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue // Skip invalid lines
		}
		buildInfo[parts[0]] = parts[1]
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return buildInfo, nil
}
