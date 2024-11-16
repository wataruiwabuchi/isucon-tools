package main

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	gsm "github.com/bradleypeabody/gorilla-sessions-memcache"
	"github.com/go-chi/chi/v5"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/kaz/pprotein/integration/standalone"
	"github.com/samber/lo"
)

var (
	db    *sqlx.DB
	store *gsm.MemcacheStore

	// 内部IPアドレスの定義
	internalIPs = struct {
		Web1 string
		Web2 string
		DB   string
	}{
		Web1: "10.0.12.166",
		Web2: "10.0.13.5",
		DB:   "10.0.0.157",
	}
)

const (
	postsPerPage  = 20
	ISO8601Format = "2006-01-02T15:04:05-07:00"
	UploadLimit   = 10 * 1024 * 1024 // 10mb _
)

type User struct {
	ID          int       `db:"id"`
	AccountName string    `db:"account_name"`
	Passhash    string    `db:"passhash"`
	Authority   int       `db:"authority"`
	DelFlg      int       `db:"del_flg"`
	CreatedAt   time.Time `db:"created_at"`
}

type Post struct {
	ID           int       `db:"id"`
	UserID       int       `db:"user_id"`
	Body         string    `db:"body"`
	Mime         string    `db:"mime"`
	CreatedAt    time.Time `db:"created_at"`
	CommentCount int
	Comments     []Comment
	User         User
	CSRFToken    string
}

type Comment struct {
	ID        int       `db:"id"`
	PostID    int       `db:"post_id"`
	UserID    int       `db:"user_id"`
	Comment   string    `db:"comment"`
	CreatedAt time.Time `db:"created_at"`
	User      User
}

var (
	userCache      = sync.Map{}
	postCache      = make([]Post, 0, 10000)
	postCacheMutex = sync.Mutex{}
)

// CommentQueue は投稿IDごとの全コメントを保持
type CommentQueue struct {
	mu       sync.RWMutex
	comments []Comment
}

// グローバルな投稿ID -> キューのマップ
var postCommentQueues sync.Map // map[int]*CommentQueue

// キューの取得（なければ作成）
func getCommentQueue(postID int) *CommentQueue {
	queue, exists := postCommentQueues.Load(postID)
	if !exists {
		// 新規キュー作成
		newQueue := &CommentQueue{
			comments: make([]Comment, 0),
		}

		// DBから全コメントをロード
		comments, err := loadComments(postID)
		if err != nil {
			log.Printf("Failed to load comments for post %d: %v", postID, err)
		} else {
			newQueue.comments = comments
		}

		// 保存（競合する場合は既存のものを使用）
		actualQueue, _ := postCommentQueues.LoadOrStore(postID, newQueue)
		queue = actualQueue
	}
	return queue.(*CommentQueue)
}

// DBから全コメントをロード
func loadComments(postID int) ([]Comment, error) {
	var comments []Comment
	err := db.Select(&comments,
		"SELECT * FROM comments WHERE post_id = ? ORDER BY created_at DESC",
		postID)
	if err != nil {
		return nil, err
	}
	return comments, nil
}

// キューにコメントを追加
func (q *CommentQueue) AddComment(comment Comment) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 新しいコメントを先頭に追加
	q.comments = append([]Comment{comment}, q.comments...)
}

// キューからコメントを取得
func (q *CommentQueue) GetComments(limit int) []Comment {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if limit <= 0 || limit >= len(q.comments) {
		result := make([]Comment, len(q.comments))
		copy(result, q.comments)
		return result
	}

	result := make([]Comment, limit)
	copy(result, q.comments[:limit])
	return result
}

// コメント数を取得
func (q *CommentQueue) GetCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.comments)
}

func init() {
	memdAddr := os.Getenv("ISUCONP_MEMCACHED_ADDRESS")
	if memdAddr == "" {
		memdAddr = "localhost:11211"
	}
	memcacheClient := memcache.New(memdAddr)
	store = gsm.NewMemcacheStore(memcacheClient, "iscogram_", []byte("sendagaya"))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func dbInitialize() {
	sqls := []string{
		"DELETE FROM users WHERE id > 1000",
		"DELETE FROM posts WHERE id > 10000",
		"DELETE FROM comments WHERE id > 100000",
		"UPDATE users SET del_flg = 0",
		"UPDATE users SET del_flg = 1 WHERE id % 50 = 0",
	}

	for _, sql := range sqls {
		db.Exec(sql)
	}
}

func tryLogin(accountName, password string) *User {
	u := User{}
	err := db.Get(&u, "SELECT * FROM users WHERE account_name = ? AND del_flg = 0", accountName)
	if err != nil {
		return nil
	}

	if calculatePasshash(u.AccountName, password) == u.Passhash {
		return &u
	} else {
		return nil
	}
}

func validateUser(accountName, password string) bool {
	return regexp.MustCompile(`\A[0-9a-zA-Z_]{3,}\z`).MatchString(accountName) &&
		regexp.MustCompile(`\A[0-9a-zA-Z_]{6,}\z`).MatchString(password)
}

// 今回のGo実装では言語側のエスケープの仕組みが使えないのでOSコマンドインジェクション対策できない
// 取り急ぎPHPのescapeshellarg関数を参考に自前で実装
// cf: http://jp2.php.net/manual/ja/function.escapeshellarg.php
func escapeshellarg(arg string) string {
	return "'" + strings.Replace(arg, "'", "'\\''", -1) + "'"
}

func digest(src string) string {
	// opensslのバージョンによっては (stdin)= というのがつくので取る
	out, err := exec.Command("/bin/bash", "-c", `printf "%s" `+escapeshellarg(src)+` | openssl dgst -sha512 | sed 's/^.*= //'`).Output()
	if err != nil {
		log.Print(err)
		return ""
	}

	return strings.TrimSuffix(string(out), "\n")
}

func calculateSalt(accountName string) string {
	return digest(accountName)
}

func calculatePasshash(accountName, password string) string {
	return digest(password + ":" + calculateSalt(accountName))
}

func getSession(r *http.Request) *sessions.Session {
	session, _ := store.Get(r, "isuconp-go.session")

	return session
}

func getSessionUser(r *http.Request) User {
	session := getSession(r)
	uid, ok := session.Values["user_id"]
	if !ok || uid == nil {
		return User{}
	}

	u := User{}

	if cachedUser, ok := userCache.Load(uid); ok {
		u = cachedUser.(User)
	} else {
		err := db.Get(&u, "SELECT * FROM `users` WHERE `id` = ?", uid)
		if err != nil {
			return User{}
		}
		userCache.Store(uid, u)
	}

	return u
}

func getFlash(w http.ResponseWriter, r *http.Request, key string) string {
	session := getSession(r)
	value, ok := session.Values[key]

	if !ok || value == nil {
		return ""
	} else {
		delete(session.Values, key)
		session.Save(r, w)
		return value.(string)
	}
}

func makePosts(results []Post, csrfToken string, allComments bool) ([]Post, error) {
	for i := range results {
		p := &results[i]

		queue := getCommentQueue(p.ID)

		if allComments {
			p.Comments = queue.GetComments(0) // 0は全件取得を意味する
		} else {
			p.Comments = queue.GetComments(3) // 最新3件のみ取得
		}

		p.CommentCount = queue.GetCount()

		// ユーザー情報を設定（既存のキャッシュロジック）
		for j := range p.Comments {
			if user, ok := userCache.Load(p.Comments[j].UserID); ok {
				p.Comments[j].User = user.(User)
			} else {
				err := db.Get(&p.Comments[j].User, "SELECT * FROM users WHERE id = ?", p.Comments[j].UserID)
				if err != nil {
					return nil, err
				}
				userCache.Store(p.Comments[j].UserID, p.Comments[j].User)
			}
		}

		p.CSRFToken = csrfToken
	}

	return results, nil
}

func imageURL(p Post) string {
	ext := mimeToExt(p.Mime)

	return "/image/" + strconv.Itoa(p.ID) + ext
}

func mimeToExt(mime string) string {
	ext := ""
	if mime == "image/jpeg" {
		ext = ".jpg"
	} else if mime == "image/png" {
		ext = ".png"
	} else if mime == "image/gif" {
		ext = ".gif"
	}

	return ext
}

func isLogin(u User) bool {
	return u.ID != 0
}

func getCSRFToken(r *http.Request) string {
	session := getSession(r)
	csrfToken, ok := session.Values["csrf_token"]
	if !ok {
		return ""
	}
	return csrfToken.(string)
}

func secureRandomStr(b int) string {
	k := make([]byte, b)
	if _, err := crand.Read(k); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", k)
}

func getTemplPath(filename string) string {
	return path.Join("templates", filename)
}

// 内部用のinitialize処理
func getInternalInitialize(w http.ResponseWriter, r *http.Request) {
	// キャッシュのクリア
	postCacheMutex.Lock()
	postCache = make([]Post, 0, 10000)
	postCacheMutex.Unlock()

	// コメントキューのクリア
	postCommentQueues = sync.Map{}

	// ユーザーキャッシュのクリア
	userCache = sync.Map{}

	w.WriteHeader(http.StatusOK)
}

func getInitialize(w http.ResponseWriter, r *http.Request) {
	dbInitialize()

	// キャッシュのクリア
	postCacheMutex.Lock()
	postCache = make([]Post, 0, 10000)
	postCacheMutex.Unlock()

	// コメントキューのクリア
	postCommentQueues = sync.Map{}

	// ユーザーキャッシュのクリア
	userCache = sync.Map{}

	// Web2の内部initializeを呼び出し
	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s:8080/internal/initialize", internalIPs.Web2))
		if err != nil {
			log.Printf("Failed to initialize web2: %v", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("Web2 initialize returned non-200 status: %d", resp.StatusCode)
			}
		}
	}()

	go func() {
		if _, err := http.Get("http://127.0.0.1:9000/api/group/collect"); err != nil {
			log.Printf("failed to communicate with pprotein: %v", err)
		}
	}()
	w.WriteHeader(http.StatusOK)
}

func getLogin(w http.ResponseWriter, r *http.Request) {
	me := getSessionUser(r)

	if isLogin(me) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	template.Must(template.ParseFiles(
		getTemplPath("layout.html"),
		getTemplPath("login.html")),
	).Execute(w, struct {
		Me    User
		Flash string
	}{me, getFlash(w, r, "notice")})
}

func postLogin(w http.ResponseWriter, r *http.Request) {
	if isLogin(getSessionUser(r)) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	u := tryLogin(r.FormValue("account_name"), r.FormValue("password"))

	if u != nil {
		session := getSession(r)
		session.Values["user_id"] = u.ID
		session.Values["csrf_token"] = secureRandomStr(16)
		session.Save(r, w)

		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		session := getSession(r)
		session.Values["notice"] = "アカウント名かパスワードが間違っています"
		session.Save(r, w)

		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func getRegister(w http.ResponseWriter, r *http.Request) {
	if isLogin(getSessionUser(r)) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	template.Must(template.ParseFiles(
		getTemplPath("layout.html"),
		getTemplPath("register.html")),
	).Execute(w, struct {
		Me    User
		Flash string
	}{User{}, getFlash(w, r, "notice")})
}

func postRegister(w http.ResponseWriter, r *http.Request) {
	if isLogin(getSessionUser(r)) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	accountName, password := r.FormValue("account_name"), r.FormValue("password")

	validated := validateUser(accountName, password)
	if !validated {
		session := getSession(r)
		session.Values["notice"] = "アカウント名は3文字以上、パスワードは6文字以上である必要があります"
		session.Save(r, w)

		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	exists := 0
	// ユーザーが存在しない場合はエラーになるのでエラーチェックはしない
	db.Get(&exists, "SELECT 1 FROM users WHERE `account_name` = ?", accountName)

	if exists == 1 {
		session := getSession(r)
		session.Values["notice"] = "アカウント名がすでに使われています"
		session.Save(r, w)

		http.Redirect(w, r, "/register", http.StatusFound)
		return
	}

	query := "INSERT INTO `users` (`account_name`, `passhash`) VALUES (?,?)"
	result, err := db.Exec(query, accountName, calculatePasshash(accountName, password))
	if err != nil {
		log.Print(err)
		return
	}

	session := getSession(r)
	uid, err := result.LastInsertId()
	if err != nil {
		log.Print(err)
		return
	}
	session.Values["user_id"] = uid
	session.Values["csrf_token"] = secureRandomStr(16)
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusFound)
}

func getLogout(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	delete(session.Values, "user_id")
	session.Options = &sessions.Options{MaxAge: -1}
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusFound)
}

func getIndex(w http.ResponseWriter, r *http.Request) {
	me := getSessionUser(r)

	postCacheMutex.Lock()
	defer postCacheMutex.Unlock()

	if len(postCache) == 0 {
		err := db.Select(&postCache, "SELECT `id`, `user_id`, `body`, `mime`, `created_at` FROM `posts` ORDER BY `created_at` DESC")
		if err != nil {
			log.Print(err)
			return
		}
	}

	posts, err := makePosts(postCache, getCSRFToken(r), false)
	if err != nil {
		log.Print(err)
		return
	}

	fmap := template.FuncMap{
		"imageURL": imageURL,
	}

	template.Must(template.New("layout.html").Funcs(fmap).ParseFiles(
		getTemplPath("layout.html"),
		getTemplPath("index.html"),
		getTemplPath("posts.html"),
		getTemplPath("post.html"),
	)).Execute(w, struct {
		Posts     []Post
		Me        User
		CSRFToken string
		Flash     string
	}{posts, me, getCSRFToken(r), getFlash(w, r, "notice")})
}

func getAccountName(w http.ResponseWriter, r *http.Request) {
	accountName := r.PathValue("accountName")
	user := User{}

	err := db.Get(&user, "SELECT * FROM `users` WHERE `account_name` = ? AND `del_flg` = 0", accountName)
	if err != nil {
		log.Print(err)
		return
	}

	if user.ID == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	postCacheMutex.Lock()
	defer postCacheMutex.Unlock()

	if len(postCache) == 0 {
		err = db.Select(&postCache, "SELECT `id`, `user_id`, `body`, `mime`, `created_at` FROM `posts` WHERE `user_id` = ? ORDER BY `created_at` DESC", user.ID)
		if err != nil {
			log.Print(err)
			return
		}
	}

	posts, err := makePosts(postCache, getCSRFToken(r), false)
	if err != nil {
		log.Print(err)
		return
	}

	commentCount := 0
	err = db.Get(&commentCount, "SELECT COUNT(*) AS count FROM `comments` WHERE `user_id` = ?", user.ID)
	if err != nil {
		log.Print(err)
		return
	}

	postIDs := lo.FilterMap(postCache, func(p Post, _ int) (int, bool) {
		if p.UserID == user.ID {
			return p.ID, true
		}
		return 0, false
	})
	postCount := len(postIDs)

	commentedCount := 0
	if postCount > 0 {
		s := []string{}
		for range postIDs {
			s = append(s, "?")
		}
		placeholder := strings.Join(s, ", ")

		// convert []int -> []interface{}
		args := make([]interface{}, len(postIDs))
		for i, v := range postIDs {
			args[i] = v
		}

		err = db.Get(&commentedCount, "SELECT COUNT(*) AS count FROM `comments` WHERE `post_id` IN ("+placeholder+")", args...)
		if err != nil {
			log.Print(err)
			return
		}
	}

	me := getSessionUser(r)

	fmap := template.FuncMap{
		"imageURL": imageURL,
	}

	template.Must(template.New("layout.html").Funcs(fmap).ParseFiles(
		getTemplPath("layout.html"),
		getTemplPath("user.html"),
		getTemplPath("posts.html"),
		getTemplPath("post.html"),
	)).Execute(w, struct {
		Posts          []Post
		User           User
		PostCount      int
		CommentCount   int
		CommentedCount int
		Me             User
	}{posts, user, postCount, commentCount, commentedCount, me})
}

func getPosts(w http.ResponseWriter, r *http.Request) {
	m, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}
	maxCreatedAt := m.Get("max_created_at")
	if maxCreatedAt == "" {
		return
	}

	t, err := time.Parse(ISO8601Format, maxCreatedAt)
	if err != nil {
		log.Print(err)
		return
	}

	postCacheMutex.Lock()
	defer postCacheMutex.Unlock()

	if len(postCache) == 0 {
		err = db.Select(&postCache, "SELECT `id`, `user_id`, `body`, `mime`, `created_at` FROM `posts` WHERE `created_at` <= ? ORDER BY `created_at` DESC", t.Format(ISO8601Format))
		if err != nil {
			log.Print(err)
			return
		}
	}

	posts, err := makePosts(postCache, getCSRFToken(r), false)
	if err != nil {
		log.Print(err)
		return
	}

	if len(posts) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	fmap := template.FuncMap{
		"imageURL": imageURL,
	}

	template.Must(template.New("posts.html").Funcs(fmap).ParseFiles(
		getTemplPath("posts.html"),
		getTemplPath("post.html"),
	)).Execute(w, posts)
}

func getPostsID(w http.ResponseWriter, r *http.Request) {
	pidStr := r.PathValue("id")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	postCacheMutex.Lock()
	defer postCacheMutex.Unlock()
	if len(postCache) == 0 {
		err = db.Select(&postCache, "SELECT * FROM `posts` WHERE `id` = ?", pid)
		if err != nil {
			log.Print(err)
			return
		}
	}
	results := lo.Filter(postCache, func(p Post, _ int) bool {
		return p.ID == pid
	})

	posts, err := makePosts(results, getCSRFToken(r), true)
	if err != nil {
		log.Print(err)
		return
	}

	if len(posts) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p := posts[0]

	me := getSessionUser(r)

	fmap := template.FuncMap{
		"imageURL": imageURL,
	}

	template.Must(template.New("layout.html").Funcs(fmap).ParseFiles(
		getTemplPath("layout.html"),
		getTemplPath("post_id.html"),
		getTemplPath("post.html"),
	)).Execute(w, struct {
		Post Post
		Me   User
	}{p, me})
}

// 新しい構造体を追加
type InternalPostRequest struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Mime      string    `json:"mime"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// 内部APIのエンドポイントを追加
func postInternalNewPost(w http.ResponseWriter, r *http.Request) {
	// 内部APIなので、Web1からのリクエストのみ許可
	clientIP := strings.Split(r.RemoteAddr, ":")[0]
	if clientIP != internalIPs.Web1 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// リクエストボディの読み取り
	var req InternalPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// postCacheに追加
	postCacheMutex.Lock()
	defer postCacheMutex.Unlock()
	postCache = append(postCache, Post{
		ID:        req.ID,
		UserID:    req.UserID,
		Mime:      req.Mime,
		Body:      req.Body,
		CreatedAt: req.CreatedAt,
	})

	w.WriteHeader(http.StatusOK)
}

// 新しい構造体を追加
type InternalCommentRequest struct {
	ID        int       `json:"id"`
	PostID    int       `json:"post_id"`
	UserID    int       `json:"user_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	User      User      `json:"user"`
}

// 内部APIのエンドポイントを追加
func postInternalNewComment(w http.ResponseWriter, r *http.Request) {
	// 内部APIなので、Web1からのリクエストのみ許可
	clientIP := strings.Split(r.RemoteAddr, ":")[0]
	if clientIP != internalIPs.Web1 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// リクエストボディの読み取り
	var req InternalCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// コメントキューに追加
	queue := getCommentQueue(req.PostID)
	queue.AddComment(Comment{
		ID:        req.ID,
		PostID:    req.PostID,
		UserID:    req.UserID,
		Comment:   req.Comment,
		CreatedAt: req.CreatedAt,
		User:      req.User,
	})

	w.WriteHeader(http.StatusOK)
}

// postIndex関数を修正
func postIndex(w http.ResponseWriter, r *http.Request) {
	me := getSessionUser(r)
	if !isLogin(me) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.FormValue("csrf_token") != getCSRFToken(r) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		session := getSession(r)
		session.Values["notice"] = "画像が必須です"
		session.Save(r, w)

		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	mime := ""
	if file != nil {
		// 投稿のContent-Typeからファイルのタイプを決定する
		contentType := header.Header["Content-Type"][0]
		if strings.Contains(contentType, "jpeg") {
			mime = "image/jpeg"
		} else if strings.Contains(contentType, "png") {
			mime = "image/png"
		} else if strings.Contains(contentType, "gif") {
			mime = "image/gif"
		} else {
			session := getSession(r)
			session.Values["notice"] = "投稿できる画像形式はjpgとpngとgifだけです"
			session.Save(r, w)

			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}

	filedata, err := io.ReadAll(file)
	if err != nil {
		log.Print(err)
		return
	}

	if len(filedata) > UploadLimit {
		session := getSession(r)
		session.Values["notice"] = "ファイルサイズが大きすぎます"
		session.Save(r, w)

		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	query := "INSERT INTO `posts` (`user_id`, `mime`, `body`) VALUES (?,?,?)"
	result, err := db.Exec(
		query,
		me.ID,
		mime,
		r.FormValue("body"),
	)
	if err != nil {
		log.Print(err)
		return
	}

	pid, err := result.LastInsertId()
	if err != nil {
		log.Print(err)
		return
	}

	ext := mimeToExt(mime)
	f, err := os.Create(fmt.Sprintf("/home/isucon/private_isu/webapp/images/%d%s", pid, ext))
	if err != nil {
		log.Print(err)
		return
	}
	defer f.Close()

	_, err = f.Write(filedata)
	if err != nil {
		log.Print(err)
		return
	}

	postCacheMutex.Lock()
	defer postCacheMutex.Unlock()
	postCache = append(postCache, Post{ID: int(pid), UserID: me.ID, Body: r.FormValue("body")})

	// Web2に投稿を通知（同期的）
	newPost := InternalPostRequest{
		ID:        int(pid),
		UserID:    me.ID,
		Mime:      mime,
		Body:      r.FormValue("body"),
		CreatedAt: time.Now(),
	}

	jsonData, err := json.Marshal(newPost)
	if err != nil {
		log.Printf("Failed to marshal post data: %v", err)
	} else {
		resp, err := http.Post(
			fmt.Sprintf("http://%s:8080/internal/post", internalIPs.Web2),
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			log.Printf("Failed to notify web2: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("Web2 returned non-200 status: %d", resp.StatusCode)
			}
		}
	}

	http.Redirect(w, r, "/posts/"+strconv.FormatInt(pid, 10), http.StatusFound)
}

func postComment(w http.ResponseWriter, r *http.Request) {
	me := getSessionUser(r)
	if !isLogin(me) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.FormValue("csrf_token") != getCSRFToken(r) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	postID, err := strconv.Atoi(r.FormValue("post_id"))
	if err != nil {
		log.Print("post_idは整数のみです")
		return
	}

	// 新しいコメントを作成
	comment := Comment{
		PostID:    postID,
		UserID:    me.ID,
		Comment:   r.FormValue("comment"),
		CreatedAt: time.Now(),
		User:      me,
	}

	// DBに保存
	result, err := db.Exec(
		"INSERT INTO comments (post_id, user_id, comment, created_at) VALUES (?,?,?,?)",
		comment.PostID, comment.UserID, comment.Comment, comment.CreatedAt)
	if err != nil {
		log.Print(err)
		return
	}

	id, _ := result.LastInsertId()
	comment.ID = int(id)

	// Web2にコメントを通知（同期的）
	newComment := InternalCommentRequest{
		ID:        comment.ID,
		PostID:    comment.PostID,
		UserID:    comment.UserID,
		Comment:   comment.Comment,
		CreatedAt: comment.CreatedAt,
		User:      comment.User,
	}

	jsonData, err := json.Marshal(newComment)
	if err != nil {
		log.Printf("Failed to marshal comment data: %v", err)
	} else {
		resp, err := http.Post(
			fmt.Sprintf("http://%s:8080/internal/comment", internalIPs.Web2),
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			log.Printf("Failed to notify web2: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("Web2 returned non-200 status: %d", resp.StatusCode)
			}
		}
	}

	// 自身のキューにコメントを追加
	queue := getCommentQueue(postID)
	queue.AddComment(comment)

	http.Redirect(w, r, fmt.Sprintf("/posts/%d", postID), http.StatusFound)
}

func getAdminBanned(w http.ResponseWriter, r *http.Request) {
	me := getSessionUser(r)
	if !isLogin(me) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if me.Authority == 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	users := []User{}
	err := db.Select(&users, "SELECT * FROM `users` WHERE `authority` = 0 AND `del_flg` = 0 ORDER BY `created_at` DESC")
	if err != nil {
		log.Print(err)
		return
	}

	template.Must(template.ParseFiles(
		getTemplPath("layout.html"),
		getTemplPath("banned.html")),
	).Execute(w, struct {
		Users     []User
		Me        User
		CSRFToken string
	}{users, me, getCSRFToken(r)})
}

func postAdminBanned(w http.ResponseWriter, r *http.Request) {
	me := getSessionUser(r)
	if !isLogin(me) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if me.Authority == 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.FormValue("csrf_token") != getCSRFToken(r) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	query := "UPDATE `users` SET `del_flg` = ? WHERE `id` = ?"

	err := r.ParseForm()
	if err != nil {
		log.Print(err)
		return
	}
	for _, id := range r.Form["uid[]"] {
		db.Exec(query, 1, id)
		if cachedUser, ok := userCache.Load(id); ok {
			user := cachedUser.(User)
			user.DelFlg = 1
			userCache.Store(id, user)
		}
	}

	http.Redirect(w, r, "/admin/banned", http.StatusFound)
}

func main() {
	host := os.Getenv("ISUCONP_DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("ISUCONP_DB_PORT")
	if port == "" {
		port = "3306"
	}
	_, err := strconv.Atoi(port)
	if err != nil {
		log.Fatalf("Failed to read DB port number from an environment variable ISUCONP_DB_PORT.\nError: %s", err.Error())
	}
	user := os.Getenv("ISUCONP_DB_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("ISUCONP_DB_PASSWORD")
	dbname := os.Getenv("ISUCONP_DB_NAME")
	if dbname == "" {
		dbname = "isuconp"
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		user,
		password,
		host,
		port,
		dbname,
	)

	db, err = sqlx.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %s.", err.Error())
	}
	defer db.Close()

	r := chi.NewRouter()

	go standalone.Integrate(":9001")

	r.Get("/initialize", getInitialize)
	r.Get("/login", getLogin)
	r.Post("/login", postLogin)
	r.Get("/register", getRegister)
	r.Post("/register", postRegister)
	r.Get("/logout", getLogout)
	r.Get("/", getIndex)
	r.Get("/posts", getPosts)
	r.Get("/posts/{id}", getPostsID)
	r.Post("/", postIndex)
	r.Post("/comment", postComment)
	r.Get("/admin/banned", getAdminBanned)
	r.Post("/admin/banned", postAdminBanned)
	r.Get(`/@{accountName:[a-zA-Z]+}`, getAccountName)
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.Dir("../public")).ServeHTTP(w, r)
	})

	// 内部APIのルートを追加
	r.Get("/internal/initialize", getInternalInitialize)
	r.Post("/internal/post", postInternalNewPost)
	r.Post("/internal/comment", postInternalNewComment)

	log.Fatal(http.ListenAndServe(":8080", r))
}
