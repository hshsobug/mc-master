// Copyright (c) 2015-2024 MinIO, Inc.
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

package cmd

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/minio/cli"
	"github.com/minio/mc/pkg/probe"
	"github.com/minio/pkg/v3/console"

	"github.com/minio/minio-go/v7"
)

// put command flags.
var (
	putFlags = []cli.Flag{
		cli.IntFlag{
			Name:  "parallel, P",
			Usage: "upload number of parts in parallel",
			Value: 4,
		},
		cli.StringFlag{
			Name:  "part-size, s",
			Usage: "each part size",
			Value: "16MiB",
		},
		cli.BoolFlag{
			Name:   "if-not-exists",
			Usage:  "upload only if object does not exist",
			Hidden: true,
		},
	}
)

// Put command.
var putCmd = cli.Command{
	Name:         "put",
	Usage:        "upload an object to a bucket",
	Action:       mainPut,
	OnUsageError: onUsageError,
	Before:       setGlobalsFromContext,
	Flags:        append(append(encFlags, globalFlags...), putFlags...),
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} [FLAGS] SOURCE TARGET

FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}

ENVIRONMENT VARIABLES:
  MC_ENC_KMS: KMS encryption key in the form of (alias/prefix=key).
  MC_ENC_S3: S3 encryption key in the form of (alias/prefix=key).

EXAMPLES:
  1. Put an object from local file system to S3 storage
     {{.Prompt}} {{.HelpName}} path-to/object play/mybucket

  2. Put an object from local file system to S3 bucket with name
     {{.Prompt}} {{.HelpName}} path-to/object play/mybucket/object

  3. Put an object from local file system to S3 bucket under a prefix
     {{.Prompt}} {{.HelpName}} path-to/object play/mybucket/object-prefix/

  4. Put an object to MinIO storage using sse-c encryption
     {{.Prompt}} {{.HelpName}} --enc-c "play/mybucket/object=MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDA" path-to/object play/mybucket/object 

  5. Put an object to MinIO storage using sse-kms encryption
     {{.Prompt}} {{.HelpName}} --enc-kms path-to/object play/mybucket/object 
`,
}

// ProgressReaderInstance is a global variable to hold the progress reader. sobug 进度监控
var ProgressReaderInstance ProgressReader
var CancelPut context.CancelFunc

// mainPut is the entry point for put command.
func mainPut(cliCtx *cli.Context) (e error) {
	// sobug
	log.Println("mainPut.")

	args := cliCtx.Args()
	if len(args) < 2 {
		showCommandHelpAndExit(cliCtx, 1) // last argument is exit code.
	}
	// sobug
	log.Printf("mc put-main.go mainPut args:%+v", args)

	var ctx context.Context
	ctx, CancelPut = context.WithCancel(globalContext)
	defer CancelPut()

	// part size
	size := cliCtx.String("s")
	if size == "" {
		size = "32MiB"
	}

	_, perr := humanize.ParseBytes(size)
	if perr != nil {
		fatalIf(probe.NewError(perr), "Unable to parse part size")
	}
	// threads
	threads := cliCtx.Int("P")
	if threads < 1 {
		fatalIf(errInvalidArgument().Trace(strconv.Itoa(threads)), "Invalid number of threads")
	}

	// Parse encryption keys per command.
	encryptionKeys, err := validateAndCreateEncryptionKeys(cliCtx)
	if err != nil {
		err.Trace(cliCtx.Args()...)
	}
	fatalIf(err, "SSE Error")

	if len(args) < 2 {
		fatalIf(errInvalidArgument().Trace(args...), "Invalid number of arguments.")
	}
	// get source and target
	// sourceURLs := args[:len(args)-1]
	// targetURL := args[len(args)-1]

	// sobug 改为单个文件上传 源路径 目标路径 目标桶
	// sourceURLs := []string{args[len(args)-3]}
	// dst := args[len(args)-2]
	// userName := args[len(args)-1]
	sourceURLs := []string{args[len(args)-4]}
	dst := args[len(args)-3]
	userName := args[len(args)-2]
	targetURL := "minio-server/" + userName

	// sobug
	log.Printf("mc put-main.go mainPut sourceURLs:%+v", sourceURLs)
	log.Printf("mc put-main.go mainPut dst:%+v", dst)
	log.Printf("mc put-main.go mainPut targetURL:%+v", targetURL)

	putURLsCh := make(chan URLs, 10000)
	var totalObjects, totalBytes int64

	// Store a progress bar or an accounter
	var pg ProgressReader

	// Enable progress bar reader only during default mode.
	if !globalQuiet && !globalJSON { // set up progress bar
		pg = newProgressBar(totalBytes)
	} else {
		pg = minio.NewAccounter(totalBytes)
	}

	// sobug 存储进度读取器初始化后赋值
	ProgressReaderInstance = pg

	go func() {
		opts := prepareCopyURLsOpts{
			sourceURLs:              sourceURLs,
			targetURL:               targetURL,
			encKeyDB:                encryptionKeys,
			ignoreBucketExistsCheck: true,
		}

		for putURLs := range preparePutURLs(ctx, opts) {
			if putURLs.Error != nil {
				putURLsCh <- putURLs
				break
			}
			totalBytes += putURLs.SourceContent.Size
			pg.SetTotal(totalBytes)
			totalObjects++
			putURLsCh <- putURLs
		}
		close(putURLsCh)
	}()
	for {
		select {
		case <-ctx.Done():
			showLastProgressBar(pg, nil)
			return
		case putURLs, ok := <-putURLsCh:
			if !ok {
				showLastProgressBar(pg, nil)
				return
			}
			if putURLs.Error != nil {
				printPutURLsError(&putURLs)
				showLastProgressBar(pg, putURLs.Error.ToGoError())
				return
			}
			urls := doCopy(ctx, doCopyOpts{
				cpURLs:           putURLs,
				pg:               pg,
				encryptionKeys:   encryptionKeys,
				multipartSize:    size,
				multipartThreads: strconv.Itoa(threads),
				ifNotExists:      cliCtx.Bool("if-not-exists"),
				dst:              "/cdata/render-data/dataserver/" + args[len(args)-1] + dst,
			})

			// 获取用户目录
			userProfile := os.Getenv("userprofile")
			// 构建目标路径
			targetDir := filepath.Join(userProfile, "scc", "NewOKs")
			targetPath := filepath.Join(targetDir, dst)

			if urls.Error != nil {
				// 上传失败
				// sobug 出错时不终止程序
				log.Println("urls.Error: ", urls.Error)
				// 出错时不finish 只报错
				showLastProgressBar(pg, urls.Error.ToGoError())
				// 出错后终止进度条,测试用 终止progressReader
				// if progressReader, ok := pg.(*progressBar); ok {
				// 	progressReader.Finish()
				// }

				// 失败的需要生成 .err 文件
				if err := createERRFile(targetPath); err != nil {
					log.Fatalf("无法创建 .err 文件: %v", err)
				} else {
					log.Printf(".err 文件创建成功: %s", targetPath)
				}

				return urls.Error.ToGoError()

				// showLastProgressBar(pg, urls.Error.ToGoError())
				// fatalIf(urls.Error.Trace(), "unable to upload")
				// return
			} else {
				// 上传成功
				log.Println("urls.Error is nil.urls", urls)
				// 假设这是上传后的文件路径
				// uploadedFilePath := "/Z/assets/12346.jpg"
				log.Println("dst:", dst)

				// 创建 .ok 文件
				if err := createOKFile(targetPath); err != nil {
					log.Fatalf("无法创建 .ok 文件: %v", err)
				} else {
					log.Printf(".ok 文件创建成功: %s", targetPath)
				}
			}
		}
	}
}

// 创建 .ok 文件
func createOKFile(filePath string) error {
	// 获取 .ok 文件的路径
	okFilePath := fmt.Sprintf("%s.ok", filePath)
	// 创建 .ok 文件所在的目录
	okFileDir := filepath.Dir(okFilePath)
	// 确保目录存在
	if err := os.MkdirAll(okFileDir, os.ModePerm); err != nil {
		return err
	}
	// 创建 .ok 文件
	file, err := os.Create(okFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 文件已经创建，不需要写入任何内容
	return nil
}

// 创建 .err 文件
func createERRFile(filePath string) error {
	// 获取 .err 文件的路径
	okFilePath := fmt.Sprintf("%s.err", filePath)
	// 创建 .err 文件所在的目录
	okFileDir := filepath.Dir(okFilePath)
	// 确保目录存在
	if err := os.MkdirAll(okFileDir, os.ModePerm); err != nil {
		return err
	}
	// 创建 .err 文件
	file, err := os.Create(okFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 文件已经创建，不需要写入任何内容
	return nil
}

func printPutURLsError(putURLs *URLs) {
	// Print in new line and adjust to top so that we
	// don't print over the ongoing scan bar
	if !globalQuiet && !globalJSON {
		console.Eraseline()
	}
	if strings.Contains(putURLs.Error.ToGoError().Error(),
		" is a folder.") {
		errorIf(putURLs.Error.Trace(),
			"Folder cannot be copied. Please use `...` suffix.")
	} else {
		errorIf(putURLs.Error.Trace(),
			"Unable to upload.")
	}
}

func showLastProgressBar(pg ProgressReader, e error) {
	if e != nil {
		// We only erase a line if we are displaying a progress bar
		if !globalQuiet && !globalJSON {
			console.Eraseline()
		}
		return
	}
	if progressReader, ok := pg.(*progressBar); ok {
		progressReader.Finish()
	} else {
		if accntReader, ok := pg.(*minio.Accounter); ok {
			log.Println("showLastProgressBar accntReader.Stat()!!!")
			printMsg(accntReader.Stat())
		}
	}
}

// GetProgressStr sobug 进度返回拼接
func GetProgressStr(pg ProgressReader) string {
	if progressReader, ok := pg.(*progressBar); ok {
		log.Println("ShowProgressReader progressBar")
		finished := "0"
		if progressReader.ProgressBar.IsFinished() {
			finished = "1"
		}
		result := strconv.Itoa(int(math.Round(progressReader.ProgressBar.GetSpeed()))) + " " + strconv.FormatInt(progressReader.ProgressBar.Get(), 10) + " " + finished
		log.Panicln("ShowProgressReader progressBar result:", result)
		return result
		//progressReader.print()
	} else {
		if accntReader, ok := pg.(*minio.Accounter); ok {
			log.Println("ShowProgressReader accntReader")
			// accntReader.print()
			finished := "0"
			select {
			case <-accntReader.IsFinished:
				// 通道已关闭，表示操作已完成
				finished = "1"
			default:
				// 通道未关闭，表示操作仍在进行
				log.Println("Operation is still ongoing")
			}
			result := strconv.Itoa(int(math.Round(accntReader.GetSpeed()))) + " " + strconv.FormatInt(accntReader.Get(), 10) + " " + finished
			log.Println("ShowProgressReader accntReader result:", result)
			return result
		} else {
			log.Println("ShowProgressReader other")
			return "ShowProgressReader other"
		}
	}

}

func CancelFilePut() {
	CancelPut()
}
