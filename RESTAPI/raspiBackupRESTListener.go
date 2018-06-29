package main

/*

 REST prototype for raspiBackup

 See https://www.linux-tips-and-tricks.de/en/backup for details about raspiBackup

 If there is any requirement for a full blown REST API please contact the author

 REST calls can be protected with userid and password. Just create a file /usr/local/etc/raspiBackup.auth
 and add lines in the format 'userid:password' to define access credetials.

 To invoke raspiBackup via REST use follwing command:
     curl -u userid:password -H "Content-Type: application/json" -X POST -d '{"target":"/backup","type":"tar", "keep": 3}' http://<raspiHost>:8080/v1/raspiBackup

(c) 2017 - framp at linux-tips-and-tricks dot de

*/

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

const (
	Executable   = "/usr/local/bin/raspiBackup.sh"
	PasswordFile = "/usr/local/etc/raspiBackup.auth"
)

type parameter struct {
	Target string  `json:"target" binding:"required"`
	Type   *string `json:"type,omitempty"`
	Keep   *int    `json:"keep,omitempty"`
}

// NoRouteHandler -
func NoRouteHandler(c *gin.Context) {
	c.JSON(404, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
}

// IndexHandler -
func IndexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

// BackupHandler - handles requests for raspiBackup
func BackupHandler(c *gin.Context) {

	var parm parameter
	err := c.BindJSON(&parm)
	if err != nil {
		msg := fmt.Sprintf("%+v", err)
		c.JSON(http.StatusBadRequest, gin.H{"Invalid payload received": msg})
		return
	}

	test := c.DefaultQuery("test", "0")
	testEnabled := test == "1"

	var args string

	if parm.Type != nil {
		args = "-t " + *parm.Type
	}

	if parm.Keep != nil {
		args += "-k " + strconv.Itoa(*parm.Keep)
	}

	args += " " + parm.Target

	command := "sudo " + Executable
	args = `"` + args + `"`
	combined := command + " " + args
	cmd := exec.Command("/bin/bash", "-c", combined)

	if !testEnabled {
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			msg := fmt.Sprintf("%+v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": msg, "output": string(stdoutStderr[:])})
		}
	}
	c.JSON(http.StatusOK, "")
}

func NewEngine(passwordSet bool, credentialMap gin.Accounts) *gin.Engine {

	gin.SetMode(gin.ReleaseMode)
	api := gin.Default()

	var v1 *gin.RouterGroup

	if passwordSet {
		v1 = api.Group("v1", gin.BasicAuth(credentialMap))
	} else {
		v1 = api.Group("v1")
	}

	api.LoadHTMLGlob("templates/*.html")
	api.Use(static.Serve("/assets", static.LocalFile("assets", false)))
	api.NoRoute(NoRouteHandler)

	v1.POST("/raspiBackup", BackupHandler)
	v1.GET("/", IndexHandler)

	return api
}

func main() {

	listenAddress := flag.String("a", ":8080", "Listen address of server. Default: :8080")
	flag.Parse()

	var passwordSet bool
	var credentialMap = map[string]string{}

	// read credentials
	if _, err := os.Stat(PasswordFile); err == nil {
		fmt.Printf("INFO: Reading %v\n", PasswordFile)
		credentials, err := ioutil.ReadFile(PasswordFile)
		if err != nil {
			fmt.Printf("%+v", err)
			os.Exit(42)
		}

		f, err := os.Open(PasswordFile)
		defer f.Close()
		if err != nil {
			log.Fatal(err)
		}

		fi, err := f.Stat()
		if err != nil {
			log.Fatal(err)
		}

		if mode := fi.Mode(); mode&077 != 0 {
			fmt.Printf("ERROR: %v not protected. %v\n", PasswordFile, mode)
			os.Exit(42)
		}

		lines := strings.Split(string(credentials), "\n")

		for i, line := range lines {
			splitCredentials := strings.Split(string(line), ":")
			if len(splitCredentials) == 2 {
				uid, pwd := strings.TrimSpace(splitCredentials[0]), strings.TrimSpace(splitCredentials[1])
				credentialMap[uid] = pwd
				fmt.Printf("INFO: Line %d: Found credential definition for userid '%s'\n", i, uid)
				passwordSet = true
			} else {
				if len(line) > 0 {
					fmt.Printf("WARN: Line %d skipped. Found '%s' which is not a valid credential definition. Expected 'userid:password'\n", i, line)
				}
			}
		}

	} else {
		fmt.Printf("WARN: REST API not protected with basic auth. %s not found\n", PasswordFile)
	}

	fmt.Printf("INFO: Server now listening on port %s\n", *listenAddress)

	api := NewEngine(passwordSet, credentialMap)

	api.Run(*listenAddress)
}
