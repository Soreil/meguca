/*
 Webserver
*/

package server

import (
	"encoding/json"
	"github.com/NYTimes/gziphandler"
	"github.com/bakape/meguca/auth"
	"github.com/bakape/meguca/config"
	"github.com/bakape/meguca/db"
	"github.com/bakape/meguca/imager"
	"github.com/bakape/meguca/server/websockets"
	"github.com/bakape/meguca/templates"
	"github.com/bakape/meguca/util"
	"github.com/dimfeld/httptreemux"
	"github.com/mssola/user_agent"
	"github.com/sebest/xff"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
)

// Used for overriding during tests
var (
	webRoot      = "www"
	imageWebRoot = "img"
	assetServer  http.Handler
)

func startWebServer() (err error) {
	conf := config.Config.HTTP
	r := createRouter()
	log.Println("Listening on " + conf.Addr)

	if conf.SSL {
		err = http.ListenAndServeTLS(conf.Addr, conf.Cert, conf.Key, r)
	} else {
		err = http.ListenAndServe(conf.Addr, r)
	}
	if err != nil {
		return util.WrapError("Error starting web server", err)
	}
	return
}

// Create the monolithic router for routing HTTP requests. Separated into own
// function for easier testability.
func createRouter() http.Handler {
	r := httptreemux.New()
	r.NotFoundHandler = notFoundPage
	r.PanicHandler = panicHandler

	// HTML
	r.GET("/", wrapHandler(redirectToDefault))
	r.GET("/all/", wrapHandler(serveIndexTemplate))
	r.GET("/:board/", boardHTML)
	r.GET("/:board/:thread", threadHTML)

	// JSON
	r.GET("/api/all/", wrapHandler(allBoardJSON))
	r.GET("/api/:board/", boardJSON)
	r.GET("/api/:board/:thread", threadJSON)
	r.GET("/api/config", wrapHandler(serveConfigs))
	r.GET("/api/post/:post", servePost)

	// Assets
	assetServer = http.FileServer(http.Dir(webRoot))
	r.GET("/ass/*path", serveAssets)
	r.GET("/img/*path", serveImages)

	// Websocket API
	r.GET("/socket", wrapHandler(websockets.Handler))

	// Image upload
	r.POST("/upload", wrapHandler(imager.NewImageUpload))

	h := http.Handler(r)
	if config.Config.HTTP.Gzip {
		h = gziphandler.GzipHandler(h)
	}
	if config.Config.HTTP.TrustProxies {
		h = xff.Handler(h)
	}

	return h
}

// Adapter for http.HandlerFunc -> httptreemux.HandlerFunc
func wrapHandler(fn http.HandlerFunc) httptreemux.HandlerFunc {
	return func(
		res http.ResponseWriter,
		req *http.Request,
		_ map[string]string,
	) {
		fn(res, req)
	}
}

// Redirects to frontpage, if set, or the default board
func redirectToDefault(res http.ResponseWriter, req *http.Request) {
	if config.Config.Frontpage != "" {
		http.ServeFile(res, req, config.Config.Frontpage)
	} else {
		http.Redirect(res, req, "/"+config.Config.Boards.Default+"/", 302)
	}
}

// Serves the standard HTML for desktop or mobile pages
func serveIndexTemplate(res http.ResponseWriter, req *http.Request) {
	isMobile := user_agent.New(req.UserAgent()).Mobile()
	var template templates.Store
	if isMobile {
		template = templates.Resources["mobile"]
	} else {
		template = templates.Resources["index"]
	}
	etag := template.Hash
	if isMobile {
		etag += "-mobile"
	}
	if !pageEtag(res, req, etag) {
		return
	}
	res.Header().Set("Content-Type", "text/html")
	writeData(res, req, template.HTML)
}

// Asserts board exists and renders the index template
func boardHTML(
	res http.ResponseWriter,
	req *http.Request,
	params map[string]string,
) {
	if auth.CanAccessBoard(params["board"], auth.LookUpIdent(req.RemoteAddr)) {
		serveIndexTemplate(res, req)
	} else {
		notFoundPage(res, req)
	}
}

// Serves board page JSON
func boardJSON(
	res http.ResponseWriter,
	req *http.Request,
	params map[string]string,
) {
	board := params["board"]
	ident := auth.LookUpIdent(req.RemoteAddr)
	if !auth.CanAccessBoard(board, ident) {
		text404(res, req)
		return
	}
	counter, err := db.BoardCounter(board)
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	if !pageEtag(res, req, etagStart(counter)) {
		return
	}
	data, err := db.NewReader(board, ident).GetBoard()
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	writeJSON(res, req, data)
}

// Asserts a thread exists on the specific board and renders the index template
func threadHTML(
	res http.ResponseWriter,
	req *http.Request,
	params map[string]string,
) {
	board := params["board"]
	id, err := strconv.ParseUint(params["thread"], 10, 64)
	if err != nil {
		notFoundPage(res, req)
		return
	}
	valid, err := validateThreadRequest(board, id)
	if err != nil {
		errorPage(res, req, err)
		return
	}
	if !valid {
		notFoundPage(res, req)
		return
	}
	serveIndexTemplate(res, req)
}

// Serves thread page JSON
func threadJSON(
	res http.ResponseWriter,
	req *http.Request,
	params map[string]string,
) {
	board := params["board"]
	id, err := strconv.ParseUint(params["thread"], 10, 64)
	if err != nil {
		text404(res, req)
		return
	}
	valid, err := validateThreadRequest(board, id)

	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	if !valid {
		notFoundPage(res, req)
		return
	}

	ident := auth.LookUpIdent(req.RemoteAddr)
	counter, err := db.ThreadCounter(id)
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	if !pageEtag(res, req, etagStart(counter)) {
		return
	}

	data, err := db.NewReader(board, ident).
		GetThread(id, detectLastN(req))
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	writeJSON(res, req, data)
}

// Cofirm thread request is proper, thread exists and client hadsright of access
func validateThreadRequest(board string, id uint64) (bool, error) {
	valid, err := db.ValidateOP(id, board)
	if err != nil {
		return false, err
	}
	return valid, nil
}

// Serves JSON for the "/all/" meta-board, that contains threads from all boards
func allBoardJSON(res http.ResponseWriter, req *http.Request) {
	counter, err := db.PostCounter()
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	if !pageEtag(res, req, etagStart(counter)) {
		return
	}

	ident := auth.LookUpIdent(req.RemoteAddr)
	data, err := db.NewReader("all", ident).GetAllBoard()
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	writeJSON(res, req, data)
}

// Build an etag for HTML or JSON pages and check if it matches the one provided
// by the client. If yes, send 304 and return false, otherwise set headers and
// return true.
func pageEtag(res http.ResponseWriter, req *http.Request, etag string) bool {
	// If etags match, no need to rerender
	if checkClientEtag(res, req, etag) {
		return false
	}
	setHeaders(res, etag)
	return true
}

// Build the main part of the etag
func etagStart(counter uint64) string {
	return "W/" + util.IDToString(counter)
}

// Check is any of the etags the client provides in the "If-None-Match" header
// match the generated etag. If yes, write 304 and return true.
func checkClientEtag(
	res http.ResponseWriter,
	req *http.Request,
	etag string,
) bool {
	if etag == req.Header.Get("If-None-Match") {
		res.WriteHeader(304)
		return true
	}
	return false
}

// Serve custom error page
func notFoundPage(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(404)
	http.ServeFile(res, req, filepath.FromSlash(webRoot+"/404.html"))
}

// Text-only 404 response
func text404(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(404)
	writeData(res, req, []byte("404 Not found"))
}

func writeData(res http.ResponseWriter, req *http.Request, data []byte) {
	_, err := res.Write(data)
	if err != nil {
		util.LogError(req.RemoteAddr, err)
	}
}

func writeJSON(res http.ResponseWriter, req *http.Request, data interface{}) {
	JSON, err := json.Marshal(data)
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	res.Header().Set("Content-Type", "aplication/json")
	writeData(res, req, JSON)
}

// Text-only 500 response
func textErrorPage(res http.ResponseWriter, req *http.Request, err error) {
	res.WriteHeader(500)
	writeData(res, req, []byte("500 Internal server error"))
}

// Cactch and log panics in webserver goroutines
func panicHandler(res http.ResponseWriter, req *http.Request, err interface{}) {
	errorPage(res, req, err.(error))
}

// Serve error page and log stack trace on error
func errorPage(res http.ResponseWriter, req *http.Request, e interface{}) {
	err := e.(error)
	res.WriteHeader(500)
	http.ServeFile(res, req, filepath.FromSlash(webRoot+"/50x.html"))
	dump, _ := httputil.DumpRequest(req, false)
	err = util.WrapError(string(dump), err)
	util.LogError(req.RemoteAddr, err)
}

var vanillaHeaders = map[string]string{
	"X-Frame-Options": "sameorigin",
	"Cache-Control":   "max-age=0, must-revalidate",
	"Expires":         "Fri, 01 Jan 1990 00:00:00 GMT",
}

// Set HTTP headers to the response object
func setHeaders(res http.ResponseWriter, etag string) {
	head := res.Header()
	for key, val := range vanillaHeaders {
		head.Set(key, val)
	}
	head.Set("ETag", etag)
}

// Validate the client's last N posts to display setting
func detectLastN(req *http.Request) int {
	query := req.URL.Query().Get("lastN")
	if query != "" {
		lastN, err := strconv.Atoi(query)
		if err == nil && lastN <= 500 {
			return lastN
		}
	}
	return 0
}

// Serve public configuration information as JSON
func serveConfigs(res http.ResponseWriter, req *http.Request) {
	etag := config.Hash
	if checkClientEtag(res, req, etag) {
		return
	}
	setHeaders(res, etag)
	writeData(res, req, config.ClientConfig)
}

// Serve a single post as JSON
func servePost(
	res http.ResponseWriter,
	req *http.Request,
	params map[string]string,
) {
	id, err := strconv.ParseUint(params["post"], 10, 64)
	if err != nil {
		text404(res, req)
		return
	}

	ident := auth.LookUpIdent(req.RemoteAddr)
	post, err := db.NewReader("", ident).GetPost(id)
	if err != nil {
		textErrorPage(res, req, err)
		return
	}

	// No post in the database or no access
	if post.ID == 0 {
		text404(res, req)
		return
	}

	data, err := json.Marshal(post)
	if err != nil {
		textErrorPage(res, req, err)
		return
	}

	etag, err := util.HashBuffer(data)
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
	if checkClientEtag(res, req, etag) {
		return
	}

	setHeaders(res, etag)
	writeData(res, req, data)
}

var imageHeaders = map[string]string{
	// max-age set to 350 days. Some caches and browsers ignore max-age, if it
	// is a year or greater, so keep it a little below.
	"Cache-Control": "max-age=30240000",

	// Fake etag to stop agressive browser cache busting
	"ETag":            "0",
	"X-Frame-Options": "sameorigin",
}

// More performant handler for serving image assets. These are immutable
// (except deletion), so we can also set seperate caching policies for them.
func serveImages(
	res http.ResponseWriter,
	req *http.Request,
	params map[string]string,
) {
	path := filepath.FromSlash(imageWebRoot + params["path"])
	file, err := os.Open(path)
	if err != nil {
		text404(res, req)
		return
	}
	defer file.Close()

	if checkClientEtag(res, req, "0") {
		return
	}
	head := res.Header()
	for key, val := range imageHeaders {
		head.Set(key, val)
	}

	_, err = io.Copy(res, file)
	if err != nil {
		textErrorPage(res, req, err)
		return
	}
}

// Server static assets
func serveAssets(
	res http.ResponseWriter,
	req *http.Request,
	params map[string]string,
) {
	req.URL.Path = params["path"]
	assetServer.ServeHTTP(res, req)
}
