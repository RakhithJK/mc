/*
 * Mini Copy (C) 2014, 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio-io/cli"
	"github.com/minio-io/mc/pkg/client"
	"github.com/minio-io/mc/pkg/console"
	"github.com/minio-io/minio/pkg/iodine"
	"github.com/minio-io/minio/pkg/utils/log"
)

const (
	printDate = "2006-01-02 15:04:05 MST"
)

// printItem prints item meta-data
func printItem(date time.Time, v int64, name string, fileType os.FileMode) {
	fmt.Printf(console.Time("[%s] ", date.Local().Format(printDate)))
	fmt.Printf(console.Size("%6s ", humanize.IBytes(uint64(v))))
	// just making it explicit
	if fileType.IsDir() {
		if strings.HasSuffix(name, "/") {
			fmt.Println(console.Dir("%s", name))
		} else {
			fmt.Println(console.Dir("%s/", name))
		}
	}
	if fileType.IsRegular() {
		fmt.Println(console.File("%s", name))
	}
}

func doListSingle(clnt client.Client, targetURL string) error {
	var err error
	for itemCh := range clnt.ListSingle() {
		if itemCh.Err != nil {
			err = itemCh.Err
			break
		}
		printItem(itemCh.Item.Time, itemCh.Item.Size, itemCh.Item.Name, itemCh.Item.FileType)
	}
	if err != nil {
		return iodine.New(err, map[string]string{"Target": targetURL})
	}
	return nil
}

func doList(clnt client.Client, targetURL string) error {
	var err error
	for itemCh := range clnt.List() {
		if itemCh.Err != nil {
			err = itemCh.Err
			break
		}
		printItem(itemCh.Item.Time, itemCh.Item.Size, itemCh.Item.Name, itemCh.Item.FileType)
	}
	if err != nil {
		return iodine.New(err, map[string]string{"Target": targetURL})
	}
	return nil
}

// runListCmd lists objects inside a bucket
func runListCmd(ctx *cli.Context) {
	if len(ctx.Args()) < 1 || ctx.Args().First() == "help" {
		cli.ShowCommandHelpAndExit(ctx, "ls", 1) // last argument is exit code
	}
	config, err := getMcConfig()
	if err != nil {
		log.Debug.Println(iodine.New(err, nil))
		console.Fatalf("Unable to read config file [%s]. Reason: [%s].\n", mustGetMcConfigPath(), iodine.ToError(err))
	}
	targetURLConfigMap := make(map[string]*hostConfig)
	for _, arg := range ctx.Args() {
		targetURL, err := getURL(arg, config.Aliases)
		if err != nil {
			switch iodine.ToError(err).(type) {
			case errUnsupportedScheme:
				log.Debug.Println(iodine.New(err, nil))
				console.Fatalf("Unknown type of URL [%s].\n", arg)
			default:
				log.Debug.Println(iodine.New(err, nil))
				console.Fatalf("Unable to parse argument [%s]. Reason: [%s].\n", arg, iodine.ToError(err))
			}
		}
		targetConfig, err := getHostConfig(targetURL)
		if err != nil {
			log.Debug.Println(iodine.New(err, nil))
			console.Fatalf("Unable to read host configuration for [%s] from config file [%s]. Reason: [%s].\n",
				targetURL, mustGetMcConfigPath(), iodine.ToError(err))
		}
		targetURLConfigMap[targetURL] = targetConfig
	}
	for targetURL, targetConfig := range targetURLConfigMap {
		if isURLRecursive(targetURL) {
			// if recursive strip off the "..."
			targetURL = strings.TrimSuffix(targetURL, recursiveSeparator)
			err = doListCmd(mcClientMethods{}, targetURL, targetConfig, globalDebugFlag)
			err = iodine.New(err, nil)
			if err != nil {
				log.Debug.Println(err)
				console.Fatalf("Failed to list [%s]. Reason: [%s].\n", targetURL, iodine.ToError(err))
			}
		} else {
			err = doListSingleCmd(mcClientMethods{}, targetURL, targetConfig, globalDebugFlag)
			if err != nil {
				if err != nil {
					log.Debug.Println(err)
					console.Fatalf("Failed to list [%s]. Reason: [%s].\n", targetURL, iodine.ToError(err))
				}
			}
		}
	}
}

func doListSingleCmd(methods clientMethods, targetURL string, targetConfig *hostConfig, debug bool) error {
	clnt, err := methods.getNewClient(targetURL, targetConfig, globalDebugFlag)
	if err != nil {
		return iodine.New(err, map[string]string{"Target": targetURL})
	}
	err = doListSingle(clnt, targetURL)
	for i := 0; i < globalMaxRetryFlag && err != nil && isValidRetry(err); i++ {
		fmt.Println(console.Retry("Retrying ... %d", i))
		// Progressively longer delays
		time.Sleep(time.Duration(i*i) * time.Second)
		err = doListSingle(clnt, targetURL)
	}
	if err != nil {
		return iodine.New(err, nil)
	}
	return nil
}

func doListCmd(methods clientMethods, targetURL string, targetConfig *hostConfig, debug bool) error {
	clnt, err := methods.getNewClient(targetURL, targetConfig, globalDebugFlag)
	if err != nil {
		return iodine.New(err, map[string]string{"Target": targetURL})
	}
	err = doList(clnt, targetURL)
	for i := 0; i < globalMaxRetryFlag && err != nil && isValidRetry(err); i++ {
		fmt.Println(console.Retry("Retrying ... %d", i))
		// Progressively longer delays
		time.Sleep(time.Duration(i*i) * time.Second)
		err = doList(clnt, targetURL)
	}
	if err != nil {
		return iodine.New(err, nil)
	}
	return nil
}
