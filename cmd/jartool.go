package cmd

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"github.com/prometheus/common/log"
	"github.com/spf13/cobra"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

var (
	jarLib bool
	jarLoc bool
	jarCmd = &cobra.Command{
		Use:   "jar subcommand [args]",
		Short: "jar包依赖分析工具",
	}
)

func init() {
	depCmd := &cobra.Command{
		Use:   "dep path",
		Short: "解析jar包依赖",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("**********解析开始***********")
			projects, err := parseEntry(args[0])
			cobra.CheckErr(err)
			log.Info("**********结果分析***********")
			for _, project := range projects {
				fmt.Printf("--[%s] [%s] [%s]\n", project.Name, project.md5sum, project.PackageAt.Format("2006-01-02 15:04:05"))
				if jarLoc {
					fmt.Printf("  %s\n", project.Path)
				}
				if jarLib {
					for _, dep := range project.Deps {
						if dep.Err == nil {
							fmt.Println("  ", dep.Name, "\t", dep.ArtifactId, "\t", dep.Version, "\t", "")
						} else {
							artifactId, version := parseArtifactIdAndVersion(dep.Name)
							fmt.Println("  ", dep.Name, "\t", artifactId, "\t", version, "\t", "?")
						}
					}
				}
			}
			log.Info("**********结束运行***********")
		},
	}

	depCmd.Flags().BoolVar(&jarLib, "jar", jarLib, "是否展示依赖")
	depCmd.Flags().BoolVar(&jarLoc, "loc", jarLoc, "是否展示真实路径")
	jarCmd.AddCommand(depCmd)

	versionCmd := &cobra.Command{
		Use:   "version path",
		Short: "解析jar包版本",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("**********解析开始***********")
			projects, err := parseEntry(args[0])
			cobra.CheckErr(err)
			log.Info("**********结果分析***********")
			latestGpArVrMap := make(map[[2]string]string)
			for _, project := range projects {
				for _, dep := range project.Deps {
					if !strings.HasSuffix(dep.ArtifactId, "service") {
						continue
					}
					gpArKey := [2]string{dep.GroupId, dep.ArtifactId}
					if vr, ok := latestGpArVrMap[gpArKey]; ok {
						if VersionCompare(dep.Version, vr) > 0 {
							latestGpArVrMap[gpArKey] = dep.Version
						}
					} else {
						latestGpArVrMap[gpArKey] = dep.Version
					}
				}
			}
			for _, project := range projects {
				buffer := bytes.Buffer{}
				for _, dep := range project.Deps {
					gpArKey := [2]string{dep.GroupId, dep.ArtifactId}
					if vr, ok := latestGpArVrMap[gpArKey]; ok {
						if VersionCompare(vr, dep.Version) > 0 {
							buffer.WriteString(fmt.Sprintf("\t%s %s %s=>%s\n", dep.GroupId, dep.ArtifactId, dep.Version, vr))
						}
					}

				}
				if buffer.Len() > 0 {
					fmt.Printf("项目%s 编译时间:%s,当前升级推荐\n%s", project.Name, project.PackageAt.Format("2006-01-02 15:04:05"), buffer.String())
				}
			}

			log.Info("**********结束运行***********")
		},
	}
	jarCmd.AddCommand(versionCmd)

	useCmd := &cobra.Command{
		Use:   "use path keyword",
		Short: "判断jar包依赖使用情况",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("**********解析开始***********")
			projects, err := parseEntry(args[0])
			cobra.CheckErr(err)
			log.Info("**********结果分析***********")
			for _, project := range projects {
				buffer := bytes.Buffer{}
				for _, dep := range project.Deps {
					if strings.Contains(dep.Name, args[1]) {
						buffer.WriteString(fmt.Sprintf("  %s\n", dep.Name))
					}
				}
				if buffer.Len() > 0 {
					fmt.Printf("--[%s] [%s] [%s]\n", project.Name, project.md5sum, project.PackageAt.Format("2006-01-02 15:04:05"))
					if jarLoc {
						fmt.Printf("  %s\n", project.Path)
					}
					fmt.Print(buffer.String())
				}
			}
			log.Info("**********结束运行***********")
		},
	}
	useCmd.Flags().BoolVar(&jarLoc, "loc", jarLoc, "是否展示真实路径")
	jarCmd.AddCommand(useCmd)

	serviceCmd := &cobra.Command{
		Use:   "shellgen jarfile",
		Short: "生成jar包管理脚本",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("**********解析开始***********")
			_, err := os.Stat(args[0])
			cobra.CheckErr(err)
			path, err := filepath.Abs(args[0])
			cobra.CheckErr(err)
			if !strings.HasSuffix(path, ".jar") && strings.HasSuffix(path, ".war") {
				log.Fatalln("文件不是java可运行文件 .jar or .war")
			}
			shellFilePath := filepath.Join(filepath.Dir(path), "service.sh")
			_, err = os.Stat(shellFilePath)
			if os.IsExist(err) {
				log.Fatalln(shellFilePath, "已经存在")
			}
			confDirPath := filepath.Join(filepath.Dir(path), "config")
			_, err = os.Stat(confDirPath)
			if os.IsNotExist(err) {
				_ = os.Mkdir(confDirPath, os.ModePerm)
				log.Info("创建配置文件目录:", confDirPath)
			}

			filename := filepath.Base(path)

			tmp, err := template.ParseFS(fileSystem, "assets/*")
			cobra.CheckErr(err)

			shellFilePathHandler, err := os.OpenFile(shellFilePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, os.ModeAppend|os.ModePerm)
			cobra.CheckErr(err)
			defer shellFilePathHandler.Close()
			err = tmp.Lookup("service.sh").Execute(shellFilePathHandler, struct {
				ProjectFileName string
			}{filename})
			cobra.CheckErr(err)
			log.Info("生成命令文件:", shellFilePath)
			log.Info("**********结束运行***********")
		},
	}
	jarCmd.AddCommand(serviceCmd)
}

func parseEntry(path string) ([]Project, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	projects := make([]Project, 0)
	if stat.IsDir() {
		err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if !strings.HasSuffix(d.Name(), ".jar") {
				return nil
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			project, err := parseProject(abs)
			if err != nil {
				return err
			}
			projects = append(projects, project)
			return nil
		})
	} else {
		if !strings.HasSuffix(stat.Name(), ".jar") {
			return nil, errors.New("不是一个jar文件")
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		project, err := parseProject(abs)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, nil
}

// parseArtifactIdAndVersion 通过名称尝试解析组件名称
func parseArtifactIdAndVersion(name string) (string, string) {
	name = strings.TrimSuffix(name, ".jar")
	dotIndex := strings.Index(name, ".")
	if dotIndex < 0 {
		return "unknown", "unknown"
	}
	index := strings.LastIndex(name[:dotIndex], "-")
	if index < 0 {
		return "unknown", "unknown"
	}
	return name[:index], name[index+1:]
}

func parseProject(path string) (Project, error) {
	project := Project{
		Name: filepath.Base(path),
		Path: path,
	}
	log.Info("解析[%s]\n", project.Path)
	md5Sum, err := Md5Sum(path)
	if err != nil {
		project.Err = err
		return project, nil
	}
	project.md5sum = md5Sum
	archive, err := zip.OpenReader(path)
	if err != nil {
		project.Err = err
		return project, nil
	}
	defer archive.Close()
	for _, ae := range archive.File {
		if strings.HasSuffix(ae.Name, "pom.properties") {
			props, err := ConvertPropertiesToMap(ae)
			if err != nil {
				log.Warn("pom.properties损坏,跳过读取!")
				project.Err = errors.New("pom.properties损坏")
				continue
			}
			project.GroupId = props["groupId"]
			project.ArtifactId = props["artifactId"]
			project.Version = props["version"]
		} else if strings.HasSuffix(ae.Name, "MANIFEST.MF") {
			project.PackageAt = ae.Modified
		} else if strings.HasSuffix(ae.Name, ".jar") {
			jarArchive, err := ZipFileToReader(ae)
			if err != nil {
				project.Deps = append(project.Deps, Dep{
					Name: filepath.Base(ae.Name),
					Path: ae.Name,
					Err:  errors.New("依赖损坏"),
				})
				log.Warn("依赖损坏,跳过读取!", err)
				continue
			}
			pomFound := false
			for _, jfe := range jarArchive.File {
				if strings.HasSuffix(jfe.Name, "pom.properties") {
					props, err := ConvertPropertiesToMap(jfe)
					if err != nil {
						project.Deps = append(project.Deps, Dep{
							Name: filepath.Base(ae.Name),
							Path: ae.Name,
							Err:  errors.New("properties损坏,跳过读取!"),
						})
						log.Warn("properties损坏,跳过读取!")
						continue
					}
					project.Deps = append(project.Deps, Dep{
						Name:       filepath.Base(ae.Name),
						Path:       ae.Name,
						GroupId:    props["groupId"],
						ArtifactId: props["artifactId"],
						Version:    props["version"],
					})
					pomFound = true
					break
				}
			}
			if !pomFound {
				project.Deps = append(project.Deps, Dep{
					Name: filepath.Base(ae.Name),
					Path: ae.Name,
					Err:  errors.New("非Maven编译项目"),
				})
			}
		}
	}
	return project, nil
}

type Dep struct {
	Name       string
	Path       string
	GroupId    string
	ArtifactId string
	Version    string
	Err        error
}

type Project struct {
	Name       string
	Path       string
	GroupId    string
	ArtifactId string
	Version    string
	PackageAt  time.Time
	md5sum     string
	Deps       []Dep
	Err        error
}
type ProjectData struct {
	ProjectFileName string
}
