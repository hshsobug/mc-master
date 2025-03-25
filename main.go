// Copyright (c) 2015-2021 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main // import "github.com/minio/mc"

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/divan/gorilla-xmlrpc/xml"
	"github.com/gorilla/rpc"

	mc "github.com/minio/mc/cmd"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var GlobalHTTPParameters HTTPParameters
var exePath string

func main() {
	// 获取exe绝对路径
	exePath, err := os.Executable()
	if err != nil {
		log.Println("Executable Error:", err)
		return
	}
	log.Println("Executable exePath:", exePath)

	// 打开日志文件，如果不存在则创建，如果存在则追加内容
	// logFile, err := os.OpenFile("mc_logs.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	// if err != nil {
	// 	log.Fatalf("无法打开日志文件: %s", err)
	// }
	// defer logFile.Close()
	// 设置日志输出位置为指定的日志文件
	//log.SetOutput(logFile)

	log.SetOutput(os.Stdout)
	// 设置日志格式，包括日期、时间和文件名
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	// 为日志消息添加前缀
	log.SetPrefix("sc_mc.exe: ")

	// 获取命令行参数 ./mc.exe 6666
	log.Println("os.Args:", os.Args)
	port := os.Args[1]
	// 集成xmlrpc
	RPC := rpc.NewServer()
	xmlrpcCodec := xml.NewCodec()
	RPC.RegisterCodec(xmlrpcCodec, "text/xml")
	RPC.RegisterService(new(HTTPService), "")
	http.Handle("/xmlrpc/", RPC)

	// 将 http.ListenAndServe 放在 goroutine 中运行
	// go func() {
	log.Println("Starting XML-RPC server on localhost:" + port)
	log.Println(http.ListenAndServe(":"+port, nil))
	// }()

}

type HTTPService struct{}
type HTTPParameters struct {
	Username           string
	Password           string
	Host               string
	Port               int
	ProxyType          int
	ProxyHost          string
	ProxyPort          int
	ProxyUser          string
	ProxyPwd           string
	Compression        bool
	Verbose            bool
	Minioadminuser     string
	Minioadminpassword string
	BucketIsExists     bool
}
type HTTPResponse struct {
	Message string
}
type HTTPParametersLog struct {
	Username           string
	Password           string
	Host               string
	Port               int
	ProxyType          int
	ProxyHost          string
	ProxyPort          int
	ProxyUser          string
	ProxyPwd           string
	Compression        bool
	Verbose            bool
	Minioadminuser     string
	Minioadminpassword string
	BucketIsExists     bool
}

var realUserName string

type STSCredentialParams struct {
	STSEndpoint       string        // STS 服务地址（示例: "http://localhost:9000"）
	LDAPUsername      string        // LDAP 用户名（必填）
	LDAPPassword      string        // LDAP 密码（必填）
	SessionPolicyPath string        // 会话策略文件路径（可选）
	ExpiryDuration    time.Duration // 凭证有效期（可选，如 1h）
}

// 新增全局变量存储凭证和有效期
var (
	credentialCache struct {
		AccessKey    string
		SecretKey    string
		SessionToken string
		ExpiryTime   time.Time
		RefreshTime  time.Time // 最后刷新时间
	}
	credentialMutex sync.Mutex // 保证凭证更新的线程安全
)

func GenerateSTSCredentials(params STSCredentialParams) (ak, sk, st string, err error) {
	var opts []credentials.LDAPIdentityOpt

	// 处理会话策略文件
	if params.SessionPolicyPath != "" {
		policy, err := os.ReadFile(params.SessionPolicyPath)
		if err != nil {
			return "", "", "", fmt.Errorf("读取策略文件失败: %v", err)
		}
		opts = append(opts, credentials.LDAPIdentityPolicyOpt(string(policy)))
	}

	// 设置凭证有效期
	if params.ExpiryDuration != 0 {
		opts = append(opts, credentials.LDAPIdentityExpiryOpt(params.ExpiryDuration))
	}

	// 生成 LDAP STS 凭证
	identity, err := credentials.NewLDAPIdentity(
		params.STSEndpoint,
		params.LDAPUsername,
		params.LDAPPassword,
		opts...,
	)
	if err != nil {
		return "", "", "", fmt.Errorf("初始化 LDAP 身份失败: %v", err)
	}

	// 获取临时凭证
	creds, err := identity.Get()
	if err != nil {
		return "", "", "", fmt.Errorf("获取凭证失败: %v", err)
	}

	return creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken, nil
}

// SftpSetParameters - 设置登录参数
func (s *HTTPService) SftpSetParameters(r *http.Request, args *HTTPParameters, reply *HTTPResponse) error {
	reply.Message = ""
	// 更新设置变量,重置设置标志
	GlobalHTTPParameters = *args
	// 创建一个新的结构体，将敏感字段替换为*
	logParams := HTTPParametersLog{
		Username:           GlobalHTTPParameters.Username,
		Password:           "*",
		Host:               GlobalHTTPParameters.Host,
		Port:               GlobalHTTPParameters.Port,
		ProxyType:          GlobalHTTPParameters.ProxyType,
		ProxyHost:          GlobalHTTPParameters.ProxyHost,
		ProxyPort:          GlobalHTTPParameters.ProxyPort,
		ProxyUser:          GlobalHTTPParameters.ProxyUser,
		ProxyPwd:           "*",
		Compression:        GlobalHTTPParameters.Compression,
		Verbose:            GlobalHTTPParameters.Verbose,
		Minioadminuser:     GlobalHTTPParameters.Minioadminuser,
		Minioadminpassword: "*",
		BucketIsExists:     GlobalHTTPParameters.BucketIsExists,
	}

	// 打印新的结构体
	log.Printf("Received parameters: %+v", logParams)

	// 为了匹配桶名规则，_转为-，大写转为小写
	realUserName = GlobalHTTPParameters.Username
	GlobalHTTPParameters.Username = strings.ToLower(strings.ReplaceAll(GlobalHTTPParameters.Username, "_", "-"))
	log.Println("finnal bucketName.", GlobalHTTPParameters.Username)

	// 拼接字符串
	newAlias := fmt.Sprintf("%s_%s", GlobalHTTPParameters.Username, strings.ReplaceAll(GlobalHTTPParameters.Host, ".", "_"))

	credentialMutex.Lock()
	defer credentialMutex.Unlock()

	// 检查缓存凭证是否有效（有效期剩余>1天且未手动失效）
	if mc.Alias == newAlias && time.Now().Before(credentialCache.ExpiryTime.Add(-24*time.Hour)) {
		log.Println("使用缓存的STS凭证")
		mc.Alias = newAlias
		return nil
	}

	// 生成7天有效期的STS凭证
	// 获取可执行文件所在目录
	// exeDir := filepath.Dir(exePath)
	// policyPath := filepath.Join(exeDir, "policy.json")
	params := STSCredentialParams{
		STSEndpoint:       "http://" + args.Host + ":" + strconv.Itoa(args.Port),
		LDAPUsername:      args.Username,
		LDAPPassword:      args.Password,
		SessionPolicyPath: "",
		ExpiryDuration:    168 * time.Hour, // 7天有效期
	}

	ak, sk, st, err := GenerateSTSCredentials(params)
	if err != nil {
		reply.Message = fmt.Sprintf("生成STS凭证失败: %v", err)
		return nil
	}

	// 3. 更新凭证缓存
	credentialCache.AccessKey = ak
	credentialCache.SecretKey = sk
	credentialCache.SessionToken = st
	credentialCache.ExpiryTime = time.Now().Add(168 * time.Hour)
	credentialCache.RefreshTime = time.Now()
	log.Printf("生成新STS凭证 有效期至: %s", credentialCache.ExpiryTime.Format("2006-01-02 15:04:05"))
	log.Println("credentialCache.AccessKey", credentialCache.AccessKey)
	mc.GlobalSessionToken = credentialCache.SessionToken
	mc.Alias = newAlias

	// 4. 使用临时凭证配置MinIO客户端
	httpHostParams := []string{
		exePath, "config", "host", "add",
		mc.Alias,
		"http://" + args.Host + ":" + strconv.Itoa(args.Port),
		credentialCache.AccessKey,
		credentialCache.SecretKey, "--api", "s3v4",
	}

	if e := mc.Main(httpHostParams); e != nil {
		log.Println("MinIO配置失败:", e)
		reply.Message = e.Error()
	} else {
		log.Println("MinIO配置成功")

		// 5. 启动异步凭证刷新检查
		go func() {
			for {
				time.Sleep(1 * time.Hour) // 每小时检查一次
				credentialMutex.Lock()
				if time.Now().After(credentialCache.ExpiryTime.Add(-24 * time.Hour)) {
					log.Println("检测到凭证即将过期，自动刷新...")
					params := STSCredentialParams{
						STSEndpoint:    "http://" + args.Host + ":" + strconv.Itoa(args.Port),
						LDAPUsername:   args.Username,
						LDAPPassword:   args.Password,
						ExpiryDuration: 168 * time.Hour,
					}
					if newAK, newSK, newSt, err := GenerateSTSCredentials(params); err == nil {
						credentialCache.AccessKey = newAK
						credentialCache.SecretKey = newSK
						credentialCache.SessionToken = newSt
						credentialCache.ExpiryTime = time.Now().Add(168 * time.Hour)
						log.Println("凭证自动刷新成功")
					}
				}
				credentialMutex.Unlock()
			}
		}()
	}

	// 6. 更新全局参数（保留原始凭证用于刷新）
	// GlobalHTTPParameters = *args

	return nil
}

// HTTPTransferParameters - 传输参数
type HTTPTransferParameters struct {
	Cmd string
	Src string
	Dst string
}

// SftpStartTransfer - 处理文件传输请求
func (s *HTTPService) SftpStartTransfer(r *http.Request, args *HTTPTransferParameters, reply *HTTPResponse) error {
	log.Printf("Received transfer parameters: %+v", args)
	reply.Message = ""

	// 创建新的上传任务后清零上传进度
	mc.SuccessFileTotal = 0
	mc.SuccessFileNum = 0

	// 在这里处理文件传输逻辑
	// 根据args.Cmd的值（reput、put、reget、get）来决定是上传还是下载
	// args.Src和args.Dst分别表示源文件和目标文件
	// 多个文件用 | 隔开，可以根据需要分割字符串来处理多个文件

	credentialMutex.Lock()
	endpoint := GlobalHTTPParameters.Host + ":" + strconv.Itoa(GlobalHTTPParameters.Port)
	accessKeyID := credentialCache.AccessKey
	secretAccessKey := credentialCache.SecretKey
	sessionToken := credentialCache.SessionToken
	useSSL := false
	credentialMutex.Unlock()

	log.Println("GlobalHTTPParameters.Username.", GlobalHTTPParameters.Username)

	// 初始化minio客户端对象
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, sessionToken),
		Secure: useSSL,
	})
	if err != nil {
		log.Println(err)
	}

	// 上传前检查桶名是否存在，不存在则创建
	if !GlobalHTTPParameters.BucketIsExists {
		// 检查bucket是否存在
		exists, err := minioClient.BucketExists(context.Background(), GlobalHTTPParameters.Username)
		if err != nil {
			log.Println("minio BucketExists error.", err)
		}
		if exists {
			log.Printf("Bucket %s already exists\n", GlobalHTTPParameters.Username)
			GlobalHTTPParameters.BucketIsExists = exists
		} else {
			log.Printf("Bucket %s does not exist\n", GlobalHTTPParameters.Username)
			//bucket := "minio-server/" + GlobalHTTPParameters.Username
			// os.Setenv("MC_STS_TOKEN", credentialCache.SessionToken)
			// mbParams := []string{
			// 	exePath, "mb", bucket,
			// }

			// log.Println("minio mb " + bucket)
			// if e := mc.Main(mbParams); e != nil {
			// 	reply.Message = e.Error()
			// 	log.Println("minio mb error.", e)
			// } else {
			// 	log.Println("minio mb success.")
			// 	GlobalHTTPParameters.BucketIsExists = true
			// }

			// 创建存储桶（新增代码）
			err = minioClient.MakeBucket(context.Background(), GlobalHTTPParameters.Username, minio.MakeBucketOptions{
				Region: "us-east-1",
			})
			if err != nil {
				log.Println("minioClient.MakeBucket error:", err)
				// 处理存储桶已存在的情况
				exists, err := minioClient.BucketExists(context.Background(), GlobalHTTPParameters.Username)
				if err != nil {
					log.Println("BucketExists检查失败:", err)
					reply.Message = fmt.Sprintf("存储桶状态检查失败: %v", err)
					return nil
				}
				if exists {
					log.Printf("Bucket %s 已存在\n", GlobalHTTPParameters.Username)
					GlobalHTTPParameters.BucketIsExists = true
					reply.Message = ""
				} else {
					reply.Message = fmt.Sprintf("创建存储桶失败: %v", err)
					log.Println("minioClient.MakeBucket error:", err)
				}
			} else {
				log.Printf("Bucket %s 创建成功\n", GlobalHTTPParameters.Username)
				GlobalHTTPParameters.BucketIsExists = true
				reply.Message = ""
			}
		}
	}

	// 示例：打印参数并返回成功消息
	cmds := strings.Split(args.Cmd, "|")
	srcs := strings.Split(args.Src, "|")
	dsts := strings.Split(args.Dst, "|")
	log.Printf("Cmds: %v, Srcs: %v, Dsts: %v", cmds, srcs, dsts)
	// 检查cmds、srcs和dsts的长度是否相等
	if len(srcs) != len(dsts) {
		reply.Message = "The number of source files, and destination paths must be the same."
		return nil
	}
	mc.AllFileNum = int64(len(srcs))
	// 目前只支持 put
	cmds[0] = "put"
	// sobug 单个文件
	// go func() {
	// 	// 另起线程提交上传任务
	// 	uploadParams := []string{exePath, cmds[0], srcs[0], dsts[0], GlobalHTTPParameters.Username, realUserName}
	// 	if e := mc.Main(uploadParams); e != nil {
	// 		log.Println("Main upload error.", e)
	// 		reply.Message = e.Error()
	// 	} else {
	// 		log.Println("Main upload success.")
	// 	}
	// }()
	// 启动多个协程并发上传文件
	go func() {
		for index := range srcs {
			// 检查文件已上传成功不再上传
			// // 获取用户目录
			// userProfile := os.Getenv("userprofile")
			// // 构建目标路径
			// targetDir := filepath.Join(userProfile, "scc", "NewOKs")
			// targetPath := filepath.Join(targetDir, dsts[index])
			// // 检查文件是否存在
			// if _, err := os.Stat(targetPath); err == nil {
			// 	log.Printf("Success File %s already exists.", targetPath)
			// 	fileInfo, err := os.Stat(srcs[index])
			// 	if err != nil {
			// 		if os.IsNotExist(err) {
			// 			log.Fatalf("File does not exist: %s", srcs[index])
			// 		} else {
			// 			log.Fatalf("Error getting file info: %v", err)
			// 		}
			// 	}
			// 	mc.SuccessFileTotal += fileInfo.Size()
			// 	mc.SuccessFileNum++
			// 	continue
			// }
			// 构建上传命令的参数列表
			// 设置环境变量传递 SessionToken
			uploadParams := []string{exePath, cmds[0], srcs[index], dsts[index], GlobalHTTPParameters.Username, realUserName}

			if e := mc.Main(uploadParams); e != nil {
				log.Printf("Main upload error for %s: %v", srcs[index], e)
				reply.Message = e.Error()
			} else {
				log.Printf("Main upload success for %s", srcs[index])
			}
		}
	}()

	return nil
}

// HTTPGetStatParameters - 获取传输状态参数
type HTTPGetStatParameters struct {
}

// SftpGetStat - 获取传输状态
func (s *HTTPService) SftpGetStat(r *http.Request, args *HTTPGetStatParameters, reply *HTTPResponse) error {
	log.Printf("Received SftpGetStat parameters: %+v", args)
	// 获取上传文件的传输状态
	if mc.ProgressReaderInstance != nil {
		reply.Message = mc.GetProgressStr(mc.ProgressReaderInstance)
	} else {
		log.Printf("mc.ProgressReaderInstance is nil")
		reply.Message = "0 0 0"
	}
	return nil
}

// SftpStopTransfer - 停止传输
func (s *HTTPService) SftpStopTransfer(r *http.Request, args *HTTPGetStatParameters, reply *HTTPResponse) error {
	log.Printf("Received SftpStopTransfer parameters: %+v", args)
	mc.CancelFilePut() // 停止传输
	reply.Message = ""
	return nil
}
