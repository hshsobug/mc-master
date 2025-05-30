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
	"log"
	"path/filepath"
	"strings"

	"github.com/minio/cli"
	"github.com/minio/minio-go/v7"
	"github.com/minio/pkg/v3/console"
)

// get command flags.
var (
	getFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "version-id, vid",
			Usage: "get a specific version of an object",
		},
	}
)

// Get command.
var getCmd = cli.Command{
	Name:         "get",
	Usage:        "get s3 object to local",
	Action:       mainGet,
	OnUsageError: onUsageError,
	Before:       setGlobalsFromContext,
	Flags:        append(append(globalFlags, encCFlag), getFlags...),
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} [FLAGS] SOURCE TARGET

FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}

EXAMPLES:
  1. Get an object from MinIO storage to local file system
     {{.Prompt}} {{.HelpName}} play/mybucket/object path-to/object

  2. Get an object from MinIO storage using encryption
     {{.Prompt}} {{.HelpName}} --enc-c "play/mybucket/object=MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDA" play/mybucket/object path-to/object
`,
}

var CancelGet context.CancelFunc

// mainGet is the entry point for get command.
func mainGet(cliCtx *cli.Context) (e error) {
	args := cliCtx.Args()
	if len(args) < 2 {
		showCommandHelpAndExit(cliCtx, 1) // last argument is exit code.
	}

	var ctx context.Context
	ctx, CancelGet = context.WithCancel(globalContext)
	defer CancelGet()

	encryptionKeys, err := validateAndCreateEncryptionKeys(cliCtx)
	if err != nil {
		err.Trace(cliCtx.Args()...)
	}
	fatalIf(err, "unable to parse encryption keys")

	// get source and target
	// sourceURLs := args[:len(args)-1]
	// targetURL := args[len(args)-1]
	sourcePath := args[len(args)-4]
	dst := args[len(args)-3]
	userName := args[len(args)-2]
	// sourceURLs := []string{args[len(args)-4]}
	// targetURL := Alias + "/" + userName
	sourceURLs := []string{Alias + "/" + userName + sourcePath}
	targetURL := args[len(args)-3]

	// sobug
	log.Printf("mc get-main.go mainGet sourceURLs:%+v", sourceURLs)
	log.Printf("mc get-main.go mainGet dst:%+v", dst)
	log.Printf("mc get-main.go mainGet targetURL:%+v", targetURL)

	getURLsCh := make(chan URLs, 10000)
	var totalObjects, totalBytes int64

	// Store a progress bar or an accounter
	// var pg ProgressReader
	// Enable progress bar reader only during default mode.
	// if !globalQuiet && !globalJSON { // set up progress bar
	// 	pg = newProgressBar(totalBytes)
	// } else {
	// 	pg = minio.NewAccounter(totalBytes)
	// }

	var pg = minio.NewAccounter(totalBytes)
	// sobug 存储进度读取器初始化后赋值
	ProgressReaderInstance = pg

	go func() {
		opts := prepareCopyURLsOpts{
			sourceURLs:              sourceURLs,
			targetURL:               targetURL,
			encKeyDB:                encryptionKeys,
			ignoreBucketExistsCheck: true,
			versionID:               cliCtx.String("version-id"),
		}

		for getURLs := range prepareGetURLs(ctx, opts) {
			if getURLs.Error != nil {
				log.Println("getURLs.Error: ", getURLs.Error)
				getURLsCh <- getURLs
				Finished = "1"
				break
			}
			totalBytes += getURLs.SourceContent.Size
			pg.SetTotal(totalBytes)
			totalObjects++
			getURLsCh <- getURLs
		}
		close(getURLsCh)
	}()
	for {
		select {
		case <-ctx.Done():
			showLastProgressBar(pg, nil)
			return
		case getURLs, ok := <-getURLsCh:
			if !ok {
				showLastProgressBar(pg, nil)
				return
			}
			if getURLs.Error != nil {
				printGetURLsError(&getURLs)
				showLastProgressBar(pg, getURLs.Error.ToGoError())
				Finished = "1"
				return
			}
			urls := doCopy(ctx, doCopyOpts{
				cpURLs:              getURLs,
				pg:                  pg,
				encryptionKeys:      encryptionKeys,
				updateProgressTotal: true,
				dst:                 "/cdata/render-data/dataserver/" + args[len(args)-1] + sourcePath,
			})
			// 获取用户目录
			// userProfile := os.Getenv("userprofile")
			// targetDir := filepath.Join(userProfile, "scc", "NewOKs")
			// 下载的目标路径与上传不同，直接在下载文件同名路径下
			targetPath := filepath.Join(dst)
			if urls.Error != nil {
				// 下载失败
				log.Println("doCopy urls.Error: ", urls.Error)
				showLastProgressBar(pg, urls.Error.ToGoError())
				// 失败的需要生成 .err 文件
				if err := createERRFile(targetPath); err != nil {
					log.Printf("无法创建 .err 文件: %v", err)
				} else {
					log.Printf(".err 文件创建成功: %s", targetPath)
				}

				Finished = "1"
				ProgressReaderInstance.(*minio.Accounter).Speed = 0
				return urls.Error.ToGoError()
			} else {
				// 下载成功
				log.Println("urls.Error is nil,urls:", urls)
				// 假设这是上传后的文件路径
				// uploadedFilePath := "/Z/assets/12346.jpg"
				// log.Println("dst:", dst)

				SuccessFileTotal += ProgressReaderInstance.Get()
				pg.SetTotal(SuccessFileTotal)
				SuccessFileNum++

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

func printGetURLsError(cpURLs *URLs) {
	// Print in new line and adjust to top so that we
	// don't print over the ongoing scan bar
	if !globalQuiet && !globalJSON {
		console.Eraseline()
	}

	if strings.Contains(cpURLs.Error.ToGoError().Error(),
		" is a folder.") {
		errorIf(cpURLs.Error.Trace(),
			"Folder cannot be copied. Please use `...` suffix.")
	} else {
		errorIf(cpURLs.Error.Trace(),
			"Unable to download.")
	}
}

func CancelFileGet() {
	CancelGet()
}
