package main

import (
	"flag"
	"fmt"
	"os"
	"net/http"
	"regexp"
	"path/filepath"
	"strings"
	"path"
	"io/ioutil"
	"net"
	"log"
	"github.com/bupt1987/fcgi_client"
)

func main() {
	addr := flag.String("addr", ":9090", "http service address")
	docroot := flag.String("docroot", "/Users/junjie/www/rob/rob-server/www/", "php docroot")
	socket := flag.String("socket", "/usr/local/var/run/php7.1-fpm.sock", "fpm listen socket")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		content, err := callFpm(*docroot, *socket, r)
		if err != nil {
			fmt.Fprint(w, err)
		} else {
			fmt.Fprint(w, string(content))
		}
	})

	log.Fatal(http.ListenAndServe(*addr, nil))
}

func callFpm(docroot string, socket string, r *http.Request) ([]byte, error) {
	fcgi, err := fcgiclient.NewClient("unix", socket)
	if err != nil {
		return nil, err
	}
	env := initEnv(docroot, r)

	resp, err := fcgi.Get(env)
	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return content, err
}

func initEnv(docroot string, r *http.Request) (map[string]string) {
	env := make(map[string]string)

	isHTTPS := r.TLS != nil
	remoteAddr, remotePort, _ := net.SplitHostPort(r.RemoteAddr)
	host, serverPort, err := net.SplitHostPort(r.Host)
	if err != nil {
		if isHTTPS {
			serverPort = "443"
		} else {
			serverPort = "80"
		}
	}

	// the basic information here
	env["CONTENT_TYPE"] = r.Header.Get("Content-Type")
	env["CONTENT_LENGTH"] = r.Header.Get("Content-Length")
	env["GATEWAY_INTERFACE"] = "CGI/1.1"
	env["REMOTE_ADDR"] = remoteAddr
	env["REMOTE_PORT"] = remotePort
	env["SERVER_PORT"] = serverPort
	env["SERVER_NAME"] = host
	env["SERVER_PROTOCOL"] = r.Proto
	env["SERVER_SOFTWARE"] = "gofast"
	env["REDIRECT_STATUS"] = "200"
	env["REQUEST_SCHEME"] = r.URL.Scheme
	env["REQUEST_METHOD"] = r.Method
	env["REQUEST_URI"] = r.RequestURI
	env["QUERY_STRING"] = r.URL.RawQuery
	if r.Host != "" {
		env["HTTP_HOST"] = r.Host
	} else {
		env["HTTP_HOST"] = "127.0.0.1"
	}

	// php information
	fastcgiScriptName := r.URL.Path

	var fastcgiPathInfo string
	pathinfoRe := regexp.MustCompile(`^(.+\.php)(/?.+)$`)
	if matches := pathinfoRe.FindStringSubmatch(fastcgiScriptName); len(matches) > 0 {
		fastcgiScriptName, fastcgiPathInfo = matches[1], matches[2]
	}

	env["PATH_INFO"] = fastcgiPathInfo
	env["PATH_TRANSLATED"] = filepath.Join(docroot, fastcgiPathInfo)
	env["SCRIPT_NAME"] = fastcgiScriptName
	env["DOCUMENT_URI"] = r.URL.Path
	env["DOCUMENT_ROOT"] = docroot

	// handle directory index
	urlPath := r.URL.Path
	if strings.HasSuffix(urlPath, "/") {
		urlPath = path.Join(urlPath, "index.php")
	}
	env["SCRIPT_FILENAME"] = path.Join(docroot, urlPath)

	// http header
	for k, v := range r.Header {
		formattedKey := strings.Replace(strings.ToUpper(k), "-", "_", -1)
		if formattedKey == "CONTENT_TYPE" || formattedKey == "CONTENT_LENGTH" {
			continue
		}

		key := "HTTP_" + formattedKey
		var value string
		if len(v) > 0 {
			//   refer to https://tools.ietf.org/html/rfc7230#section-3.2.2
			//
			//   A recipient MAY combine multiple header fields with the same field
			//   name into one "field-name: field-value" pair, without changing the
			//   semantics of the message, by appending each subsequent field value to
			//   the combined field value in order, separated by a comma.  The order
			//   in which header fields with the same field name are received is
			//   therefore significant to the interpretation of the combined field
			//   value; a proxy MUST NOT change the order of these field values when
			//   forwarding a message.
			value = strings.Join(v, ",")
		}
		env[key] = value
	}

	return env
}
