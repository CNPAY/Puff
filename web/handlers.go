package web

import (
	"Puff/config"
	"Puff/core"
	"Puff/logger"
	"Puff/notification"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Puff/storage"
)

// 从main包导入版本号
var AppVersion = "v2.1.12"

// SetAppVersion 设置应用版本号（由main包调用）
func SetAppVersion(version string) {
	AppVersion = version
}

// GithubRelease GitHub发布信息
type GithubRelease struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
}

// Notifier 通知器接口（为了类型引用）
type Notifier = notification.Notifier

// handleLogin 登录处理器
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.serveLoginPage(w, r)
	case "POST":
		s.processLogin(w, r)
	default:
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
	}
}

// serveLoginPage 服务登录页面
func (s *Server) serveLoginPage(w http.ResponseWriter, r *http.Request) {
	loginHTML := `
<!DOCTYPE html>
<html lang="zh-CN" data-theme="lofi">
<head>
    <title>Puff - 登录</title>
	<link rel="icon" href="data:image/svg+xml;base64,PHN2ZyB0PSIxNzY1OTA3NjEzNzgzIiBjbGFzcz0iaWNvbiIgdmlld0JveD0iMCAwIDE0NDggMTAyNCIgdmVyc2lvbj0iMS4xIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHAtaWQ9IjYzMDIiIHdpZHRoPSIxMjgiIGhlaWdodD0iMTI4Ij48cGF0aCBkPSJNMTAzNi45MDEwNTggNzc5LjU2Nzk0OWMzNS40MzQ5MDctMTIuOTAxMDU4IDYzLjQ3MzIwNy00MC43NjczNDQgNzYuMDMwMjM3LTc2LjM3NDI2NSAxMi4wNDA5ODgtMzMuODg2NzggMTguNzQ5NTM4LTcwLjE4MTc1NyAxOC43NDk1MzgtMTA3LjY4MDgzMyAwLTE5Mi42NTU4MDQtMTcxLjQ5ODA2OC0zNDguNjcyNjAyLTM4Mi45MDM0MS0zNDguNjcyNjAycy0zODIuOTAzNDEgMTU2LjE4ODgxMi0zODIuOTAzNDEgMzQ4LjY3MjYwMmMwIDMzLjg4Njc4IDUuMzMyNDM3IDY2Ljc0MTQ3NSAxNS4zMDkyNTYgOTcuODc2MDI5IDExLjY5Njk2IDM2LjQ2Njk5MSA0MC4wNzkyODggNjUuMzY1MzYyIDc2LjIwMjI1MSA3OC4yNjY0MiAxMzQuMzQzMDIgNDcuOTkxOTM3IDM0OC4zMjg1NzQgOTEuMzM5NDkzIDU3OS41MTU1MzggNy45MTI2NDl6IiBmaWxsPSIjRkJERUIyIiBwLWlkPSI2MzAzIj48L3BhdGg+PHBhdGggZD0iTTk2NC44MjcxNDYgNzYwLjk5MDQyNWMyNi42NjIxODctOS42MzI3OSA0Ny42NDc5MDktMzAuNjE4NTEyIDU3LjEwODY4NS01Ny4yODA2OTkgOS4xMTY3NDgtMjUuNDU4MDg4IDEzLjkzMzE0My01Mi42MzYzMTggMTMuOTMzMTQzLTgwLjg0NjYzMiAwLTE0NC40OTE4NTMtMTI4LjQ5NDU0MS0yNjEuNjMzNDYyLTI4Ny4yNjM1NjUtMjYxLjYzMzQ2MnMtMjg3LjI2MzU2NSAxMTcuMTQxNjA5LTI4Ny4yNjM1NjUgMjYxLjYzMzQ2MmMwIDI1LjQ1ODA4OCAzLjk1NjMyNSA1MC4wNTYxMDYgMTEuNTI0OTQ2IDczLjI3ODAxMSA4Ljc3MjcyIDI3LjM1MDI0NCAzMC4xMDI0NjkgNDkuMDI0MDIyIDU3LjEwODY4NSA1OC42NTY4MTIgMTAwLjk3MjI4MyAzNi4yOTQ5NzcgMjYxLjQ2MTQ0OCA2OC44MDU2NDQgNDM0Ljg1MTY3MSA2LjE5MjUwOHoiIGZpbGw9IiNGREZCRTQiIHAtaWQ9IjYzMDQiPjwvcGF0aD48cGF0aCBkPSJNOTY0LjgyNzE0NiA3NjAuOTkwNDI1YzE1Ljk5NzMxMi01Ljg0ODQ4IDMwLjI3NDQ4My0xNS45OTczMTIgNDAuOTM5MzU4LTI5LjI0MjM5OSAxMS4wMDg5MDMtMTMuMDczMDcyIDE3LjAyOTM5Ny0yOS40MTQ0MTMgMjEuMzI5NzUtNDUuNzU1NzUzIDQuNDcyMzY3LTE2LjM0MTM0MSA3LjA1MjU3OS0zMy4xOTg3MjMgNy45MTI2NDktNTAuMjI4MTIgMC4xNzIwMTQtNC4zMDAzNTMgMC4zNDQwMjgtOC40Mjg2OTEgMC4zNDQwMjgtMTIuNzI5MDQ1bC0wLjY4ODA1Ni0xMi43MjkwNDQtMC42ODgwNTctMTIuNzI5MDQ0Yy0wLjM0NDAyOC00LjMwMDM1My0xLjM3NjExMy04LjQyODY5MS0xLjg5MjE1NS0xMi41NTcwMy0xMC42NjQ4NzUtNjcuMjU3NTE3LTUxLjk0ODI2MS0xMjcuMTE4NDI4LTEwNi45OTI3NzctMTY1LjY0OTU4OS0yNy41MjIyNTgtMTkuNDM3NTk0LTU4LjMxMjc4My0zNC4wNTg3OTQtOTAuODIzNDUtNDMuMDAzNTI3LTMyLjUxMDY2Ny04Ljk0NDczNC02Ni4yMjU0MzMtMTIuMDQwOTg4LTk5Ljc2ODE4NC0xMC40OTI4NjEtNjYuOTEzNDg5IDIuOTI0MjQtMTMzLjEzODkyMiAyOC4yMTAzMTQtMTgyLjY3ODk4NSA3My4xMDU5OTctMjQuOTQyMDQ2IDIyLjE4OTgyLTQ1LjkyNzc2OCA0OC44NTIwMDctNjAuNzIwOTgxIDc4Ljc4MjQ2My0xNC43OTMyMTQgMjkuOTMwNDU1LTIzLjM5MzkxOSA2Mi43ODUxNS0yNC41OTgwMTggOTYuMTU1ODg3LTEuMjA0MDk5IDE2LjY4NTM2OSAwLjE3MjAxNCAzMy4zNzA3MzcgMi43NTIyMjYgNDkuODg0MDkzIDEuMzc2MTEzIDguMjU2Njc3IDMuMjY4MjY4IDE2LjM0MTM0MSA1LjUwNDQ1MSAyNC40MjYwMDMgMi4yMzYxODMgOC4yNTY2NzcgNC45ODg0MDkgMTUuNjUzMjg0IDguOTQ0NzM0IDIyLjg3Nzg3NyA3LjkxMjY0OSAxNC4yNzcxNzEgMTkuOTUzNjM3IDI2LjMxODE1OSAzNC4yMzA4MDggMzQuNDAyODIyIDcuMDUyNTc5IDQuMTI4MzM5IDE0Ljk2NTIyOCA2Ljg4MDU2NCAyMy4wNDk4OTEgOS42MzI3OSA3LjkxMjY0OSAyLjkyNDI0IDE1Ljk5NzMxMiA1LjUwNDQ1MiAyNC4wODE5NzUgOC4wODQ2NjMgNjQuNTA1MjkxIDIwLjI5NzY2NSAxMzIuMjc4ODUxIDMxLjgyMjYxIDIwMC4wNTI0MTEgMzEuNjUwNTk3IDMzLjg4Njc4LTAuMTcyMDE0IDY3Ljk0NTU3NC0yLjc1MjIyNiAxMDEuMzE2MzExLTguNjAwNzA2IDMzLjM3MDczNy01LjUwNDQ1MiA2Ni4yMjU0MzMtMTQuMTA1MTU3IDk4LjM5MjA3MS0yNS4yODYwNzR6IG0wIDBjLTMxLjk5NDYyNSAxMS4zNTI5MzEtNjQuODQ5MzIgMjAuMjk3NjY1LTk4LjM5MjA3MSAyNi40OTAxNzMtMzMuNTQyNzUyIDYuMDIwNDk0LTY3LjQyOTUzMSA4Ljk0NDczNC0xMDEuNDg4MzI1IDkuMjg4NzYyLTY4LjExNzU4OCAxLjAzMjA4NS0xMzYuMjM1MTc2LTkuMTE2NzQ4LTIwMS42MDA1MzgtMjguMzgyMzI4LTguMjU2Njc3LTIuNDA4MTk4LTE2LjM0MTM0MS00LjgxNjM5NS0yNC40MjYwMDQtNy41Njg2MjEtOC4wODQ2NjMtMi41ODAyMTItMTYuMzQxMzQxLTUuMzMyNDM3LTI0LjA4MTk3NS05LjYzMjc5LTE1LjMwOTI1Ni04LjQyODY5MS0yOC41NTQzNDItMjAuOTg1NzIxLTM3LjMyNzA2Mi0zNi4yOTQ5NzgtNC40NzIzNjctNy41Njg2MjEtNy43NDA2MzUtMTUuOTk3MzEyLTkuOTc2ODE5LTI0LjI1Mzk4OS0yLjU4MDIxMi04LjI1NjY3Ny00LjQ3MjM2Ny0xNi42ODUzNjktNi4xOTI1MDgtMjUuMTE0MDYtMi45MjQyNC0xNy4wMjkzOTctNC44MTYzOTUtMzQuNDAyODIyLTMuNjEyMjk2LTUxLjYwNDIzNCAxLjAzMjA4NS0zNC41NzQ4MzYgOS42MzI3OS02OC44MDU2NDQgMjQuNzcwMDMyLTk5Ljk0MDE5OHMzNi42MzkwMDYtNTkuMDAwODQgNjIuNDQxMTIyLTgyLjA1MDczYzEwNC4wNjg1MzctOTMuMjMxNjQ4IDI3Mi4xMjYzMjMtOTkuMjUyMTQyIDM4Mi43MzEzOTYtMTYuMzQxMzQxIDI3LjY5NDI3MiAyMC4yOTc2NjUgNTEuNjA0MjMzIDQ1LjQxMTcyNSA2OS44Mzc3MjkgNzQuMzEwMDk2IDE4LjIzMzQ5NiAyOC44OTgzNzEgMzAuNjE4NTEyIDYxLjQwOTAzNyAzNS43Nzg5MzUgOTUuMTIzODAzIDAuNjg4MDU2IDQuMTI4MzM5IDEuNTQ4MTI3IDguNDI4NjkxIDEuODkyMTU1IDEyLjU1NzAzbDAuNjg4MDU3IDEyLjcyOTA0NCAwLjY4ODA1NiAxMi43MjkwNDRjMCA0LjMwMDM1My0wLjE3MjAxNCA4LjYwMDcwNi0wLjM0NDAyOCAxMi43MjkwNDUtMS4wMzIwODUgMTcuMDI5Mzk3LTMuNzg0MzEgMzMuODg2NzgtOC4yNTY2NzcgNTAuMjI4MTItNC40NzIzNjcgMTYuMzQxMzQxLTEwLjQ5Mjg2MSAzMi42ODI2ODEtMjEuNjczNzc4IDQ1Ljc1NTc1My0xMS4wMDg5MDMgMTMuNDE3MTAxLTI1LjQ1ODA4OCAyMy41NjU5MzMtNDEuNDU1NDAxIDI5LjI0MjM5OXoiIGZpbGw9IiNGOUQ1OTIiIHAtaWQ9IjYzMDUiPjwvcGF0aD48cGF0aCBkPSJNNzM2LjIyMDM5MyA4MTcuMjM5MDM5Yy0xMjMuMzM0MTE3IDAtMjMyLjIxOTA0OS0yNi44MzQyMDEtMzE0Ljk1NzgzNi01Ni40MjA2MjgtNDEuOTcxNDQzLTE0Ljk2NTIyOC03NC45OTgxNTItNDkuMDI0MDIyLTg4LjQxNTI1My05MC44MjM0NS0xMC44MzY4ODktMzMuNzE0NzY2LTE2LjM0MTM0MS02OC42MzM2My0xNi4zNDEzNDEtMTAzLjg5NjUyMyAwLTIwMy4zMjA2NzkgMTgwLjYxNDgxNi0zNjguNjI2MjM5IDQwMi42ODUwMzMtMzY4LjYyNjIzOSAxOS45NTM2MzcgMCAzOS45MDcyNzQgMS4zNzYxMTMgNTkuMzQ0ODY4IDMuOTU2MzI0IDEwLjgzNjg4OSAxLjU0ODEyNyAxOC41Nzc1MjQgMTEuNTI0OTQ1IDE3LjAyOTM5NyAyMi4zNjE4MzUtMS41NDgxMjcgMTAuODM2ODg5LTExLjUyNDk0NSAxOC40MDU1MS0yMi4zNjE4MzQgMTcuMDI5Mzk3LTE3LjcxNzQ1My0yLjQwODE5OC0zNS45NTA5NDktMy42MTIyOTYtNTQuMDEyNDMxLTMuNjEyMjk3LTIwMC4yMjQ0MjUgMC0zNjIuOTQ5NzczIDE0Ny41ODgxMDctMzYyLjk0OTc3MyAzMjguODkwOTggMCAzMS4xMzQ1NTQgNC44MTYzOTUgNjEuOTI1MDggMTQuNDQ5MTg1IDkxLjY4MzUyMSA5LjYzMjc5IDMwLjI3NDQ4MyAzMy41NDI3NTIgNTQuNzAwNDg3IDYzLjk4OTI0OSA2NS41MzczNzYgMTM0LjY4NzA0OSA0OC4zMzU5NjUgMzQyLjMwODA4IDg4LjkzMTI5NSA1NjUuOTI2NDI0IDguMDg0NjYzIDMwLjEwMjQ2OS0xMC44MzY4ODkgNTMuNDk2Mzg4LTM0LjIzMDgwOCA2NC4xNjEyNjMtNjQuMzMzMjc3IDExLjY5Njk2LTMyLjUxMDY2NyAxNy41NDU0MzktNjYuNTY5NDYxIDE3LjU0NTQzOS0xMDEuMTQ0Mjk3IDAtODguOTMxMjk1LTM4LjUzMTE2MS0xNzIuMzU4MTM5LTEwOC43MTI5MTctMjM0LjYyNzI0Ny04LjI1NjY3Ny03LjIyNDU5My04Ljk0NDczNC0xOS43ODE2MjMtMS43MjAxNDEtMjguMDM4MyA3LjIyNDU5My04LjI1NjY3NyAxOS43ODE2MjMtOC45NDQ3MzQgMjguMDM4My0xLjcyMDE0MSA3OC43ODI0NjMgNzAuMDA5NzQzIDEyMS45NTgwMDQgMTYzLjkyOTQ0NyAxMjEuOTU4MDA0IDI2NC4zODU2ODggMCAzOS4wNDcyMDMtNi43MDg1NSA3Ny41NzgzNjQtMTkuNzgxNjIzIDExNC4zODkzODMtMTQuNzkzMjE0IDQxLjI4MzM4Ny00Ni43ODc4MzggNzMuNDUwMDI1LTg4LjA3MTIyNCA4OC40MTUyNTMtOTYuODQzOTQ0IDM1LjA5MDg3OS0xOTAuOTM1NjYzIDQ4LjUwNzk3OS0yNzcuODAyNzg5IDQ4LjUwNzk3OXoiIGZpbGw9IiIgcC1pZD0iNjMwNiI+PC9wYXRoPjxwYXRoIGQ9Ik04OTYuNzA5NTU4IDI3Ni43NzA3MDRjLTIuOTI0MjQgMC01Ljg0ODQ4LTAuNjg4MDU2LTguNjAwNzA1LTEuODkyMTU1LTEzLjU4OTExNS02LjUzNjUzNi0yNy44NjYyODYtMTIuMjEzMDAyLTQyLjMxNTQ3Mi0xNy4yMDE0MTEtMTAuMzIwODQ3LTMuNDQwMjgyLTE1Ljk5NzMxMi0xNC43OTMyMTQtMTIuNTU3MDMtMjUuMTE0MDYxIDMuNDQwMjgyLTEwLjQ5Mjg2MSAxNC43OTMyMTQtMTUuOTk3MzEyIDI1LjExNDA2MS0xMi41NTcwMyAxNS44MjUyOTggNS4zMzI0MzcgMzEuNjUwNTk2IDExLjY5Njk2IDQ2LjYxNTgyNCAxOC45MjE1NTMgOS45NzY4MTggNC42NDQzODEgMTQuMTA1MTU3IDE2LjUxMzM1NSA5LjQ2MDc3NiAyNi40OTAxNzMtMy4yNjgyNjggNy4wNTI1NzktMTAuMzIwODQ3IDExLjM1MjkzMS0xNy43MTc0NTQgMTEuMzUyOTMxeiIgZmlsbD0iIiBwLWlkPSI2MzA3Ij48L3BhdGg+PC9zdmc+" type="image/x-icon">
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link href="https://cdn.jsdelivr.net/npm/daisyui@4.4.24/dist/full.min.css" rel="stylesheet" type="text/css" />
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="min-h-screen bg-base-200 flex items-center justify-center">
    <div class="card w-96 bg-base-100 shadow-xl">
        <div class="card-body">
            <h1 class="card-title justify-center text-5xl mb-6">Puff</h1>
            
            <form id="loginForm" method="POST" class="space-y-4">
                <div class="form-control w-full">
                    <label class="label">
                        <span class="label-text">用户名</span>
                    </label>
                    <input type="text" name="username" class="input input-bordered w-full" required />
                </div>
                
                <div class="form-control w-full">
                    <label class="label">
                        <span class="label-text">密码</span>
                    </label>
                    <input type="password" name="password" class="input input-bordered w-full" required />
                </div>
                
                <div class="form-control w-full mt-6">
                    <button type="submit" class="btn btn-primary" id="loginBtn">
                        <span class="loading loading-spinner loading-sm hidden" id="loginSpinner"></span>
                        登录
                    </button>
                </div>
                
                <div id="errorAlert" class="alert alert-error hidden">
                    <svg xmlns="http://www.w3.org/2000/svg" class="stroke-current shrink-0 h-6 w-6" fill="none" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <span id="errorMessage"></span>
                </div>
            </form>
        </div>
    </div>

    <script>
        document.getElementById('loginForm').addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const loginBtn = document.getElementById('loginBtn');
            const loginSpinner = document.getElementById('loginSpinner');
            const errorAlert = document.getElementById('errorAlert');
            const errorMessage = document.getElementById('errorMessage');
            
            // 显示加载状态
            loginBtn.disabled = true;
            loginSpinner.classList.remove('hidden');
            errorAlert.classList.add('hidden');
            
            try {
                const formData = new FormData(this);
                const response = await fetch('/login', {
                    method: 'POST',
                    body: formData
                });
                
                if (response.ok) {
                    // 登录成功，重定向到主页面
                    window.location.href = '/';
                } else {
                    // 登录失败，显示错误信息
                    const result = await response.json();
                    errorMessage.textContent = result.error || '登录失败';
                    errorAlert.classList.remove('hidden');
                }
            } catch (error) {
                errorMessage.textContent = '网络错误，请重试';
                errorAlert.classList.remove('hidden');
            } finally {
                // 恢复按钮状态
                loginBtn.disabled = false;
                loginSpinner.classList.add('hidden');
            }
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(loginHTML))
}

// processLogin 处理登录
func (s *Server) processLogin(w http.ResponseWriter, r *http.Request) {
	// 速率限制检查
	clientIP := r.RemoteAddr
	if !s.loginLimiter.Allow(clientIP) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "登录尝试过于频繁，请5分钟后再试",
		})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	session, err := s.auth.Login(username, password)
	if err != nil {
		// 返回JSON错误响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "用户名或密码错误",
		})
		return
	}

	// 设置会话Cookie
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // 在生产环境中应该设置为true
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	}

	http.SetCookie(w, cookie)

	// 设置记住登录的Cookie，便于重启后自动恢复会话
	if rememberToken := s.auth.GenerateRememberToken(); rememberToken != "" {
		rememberCookie := &http.Cookie{
			Name:     "remember_token",
			Value:    rememberToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(s.auth.RememberDuration()),
		}
		http.SetCookie(w, rememberCookie)
	}

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// handleLogout 登出处理器
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// 获取会话ID
	if cookie, err := r.Cookie("session_id"); err == nil {
		s.auth.Logout(cookie.Value)
	}

	// 清除Cookie
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	}

	http.SetCookie(w, cookie)
	// 同步清理记住登录Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "remember_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleDomains 域名列表处理器（支持分页、搜索、过滤）
func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// 获取分页参数
		pageStr := r.URL.Query().Get("page")
		limitStr := r.URL.Query().Get("limit")
		statsOnlyStr := r.URL.Query().Get("stats_only")
		searchTerm := strings.TrimSpace(r.URL.Query().Get("search"))
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

		page := 1
		limit := 10
		statsOnly := statsOnlyStr == "true"

		if pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		// 从数据库获取域名列表（按添加时间排序）
		domainEntries, err := storage.ListDomains(false)
		if err != nil {
			s.writeError(w, "获取域名列表失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		totalAll := len(domainEntries)

		// 如果只需要统计信息，直接返回
		if statsOnly {
			s.writeJSON(w, map[string]interface{}{
				"total": totalAll,
			})
			return
		}

		// 获取所有域名的查询结果
		results, err := storage.LoadDomainResults()
		if err != nil {
			logger.Warn("加载域名结果失败: %v", err)
			results = make(map[string]storage.DomainResult)
		}

		// 按添加时间顺序构建域名信息列表
		allDomains := make([]*core.DomainInfo, 0, len(domainEntries))
		for _, entry := range domainEntries {
			domain := strings.ToLower(entry.Name)

			// 从结果中查找
			if result, ok := results[domain]; ok {
				allDomains = append(allDomains, &core.DomainInfo{
					Name:         result.Domain,
					Status:       core.DomainStatus(result.Status),
					Registrar:    result.Registrar,
					LastChecked:  result.LastChecked,
					QueryMethod:  result.QueryMethod,
					CreatedDate:  result.CreatedAt,
					ExpiryDate:   result.ExpiryAt,
					UpdatedDate:  result.UpdatedAt,
					NameServers:  result.NameServers,
					WhoisRaw:     result.WhoisRaw,
					ErrorMessage: result.ErrorMessage,
					AddedAt:      &entry.CreatedAt,
				})
			} else {
				// 没有查询结果，返回占位信息
				allDomains = append(allDomains, &core.DomainInfo{
					Name:        domain,
					Status:      core.StatusUnknown,
					LastChecked: time.Now(),
					QueryMethod: "pending",
					AddedAt:     &entry.CreatedAt,
				})
			}
		}

		// 加载域名的添加时间（从domains表）
		domainAddedAtMap := make(map[string]time.Time)
		if domainEntries, err := storage.ListDomains(false); err == nil {
			for _, entry := range domainEntries {
				domainAddedAtMap[strings.ToLower(entry.Name)] = entry.CreatedAt
			}
		}

		// 填充AddedAt字段
		for _, domain := range allDomains {
			if addedAt, ok := domainAddedAtMap[strings.ToLower(domain.Name)]; ok {
				domain.AddedAt = &addedAt
			}
		}

		// 应用搜索和状态过滤
		filteredDomains := s.filterDomains(allDomains, searchTerm, statusFilter)
		totalFiltered := len(filteredDomains)

		// 调试日志：记录筛选结果
		if statusFilter != "" || searchTerm != "" {
			logger.Debug("筛选条件: search=%s status=%s, 总数=%d, 筛选后=%d", searchTerm, statusFilter, len(allDomains), totalFiltered)
		}

		// 计算分页
		start := (page - 1) * limit
		end := start + limit

		if start >= totalFiltered {
			start = 0
			end = 0
		} else if end > totalFiltered {
			end = totalFiltered
		}

		var paginatedDomains []*core.DomainInfo
		if start < end {
			paginatedDomains = filteredDomains[start:end]
		}

		response := map[string]interface{}{
			"domains":        paginatedDomains,
			"total":          totalAll,      // 全量域名数
			"total_filtered": totalFiltered, // 过滤后数量
			"page":           page,
			"limit":          limit,
			"total_pages":    (totalFiltered + limit - 1) / limit,
			"has_next":       end < totalFiltered,
			"has_prev":       page > 1,
		}

		// 如果没有域名数据，添加明确标记
		if totalAll == 0 {
			response["data_status"] = "empty"
		} else if len(paginatedDomains) == 0 {
			response["data_status"] = "no_results"
		} else {
			response["data_status"] = "ok"
		}

		s.writeJSON(w, response)
	default:
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
	}
}

// handleDomainDetail 域名详情处理器
func (s *Server) handleDomainDetail(w http.ResponseWriter, r *http.Request) {
	// 从URL路径提取域名
	path := strings.TrimPrefix(r.URL.Path, "/api/domain/")
	domain := strings.TrimSuffix(path, "/")

	if domain == "" {
		s.writeError(w, "域名不能为空", http.StatusBadRequest)
		return
	}

	info, err := s.monitor.GetDomainInfo(domain)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, info)
}

// handleDomainCheck 域名检查处理器
func (s *Server) handleDomainCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取域名
	path := strings.TrimPrefix(r.URL.Path, "/api/domain/check/")
	domain := strings.TrimSuffix(path, "/")

	if domain == "" {
		s.writeError(w, "域名不能为空", http.StatusBadRequest)
		return
	}

	info, err := s.monitor.ForceCheck(domain)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, info)
}

// handleStats 统计信息处理器
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	stats := s.monitor.GetStats()
	authStats := s.auth.GetStats()
	notificationStats := s.notification.GetStats()

	response := map[string]interface{}{
		"monitor":      stats,
		"auth":         authStats,
		"notification": notificationStats,
		"timestamp":    time.Now(),
	}

	s.writeJSON(w, response)
}

// handleMonitorStart 启动监控处理器
func (s *Server) handleMonitorStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	if err := s.monitor.Start(); err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{"status": "started"})
}

// handleMonitorStop 停止监控处理器
func (s *Server) handleMonitorStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	s.monitor.Stop()
	s.writeJSON(w, map[string]string{"status": "stopped"})
}

// handleMonitorReload 重新加载监控处理器
func (s *Server) handleMonitorReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 重新加载域名列表
	if err := s.monitor.LoadDomains(); err != nil {
		s.writeError(w, fmt.Sprintf("重新加载域名列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{"status": "reloaded"})
}

// handleNotificationTest 通知测试处理器
func (s *Server) handleNotificationTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 构建详细的响应信息
	emailStatus := "未配置"
	telegramStatus := "未配置"
	hasAnyEnabled := false

	// 只测试已启用的通知器
	if s.config.SMTP.Enabled {
		hasAnyEnabled = true
		// 找到邮件通知器并测试
		var emailNotifier Notifier
		for _, notifier := range s.notification.GetNotifiers() {
			if notifier.GetType() == "email" && notifier.IsEnabled() {
				emailNotifier = notifier
				break
			}
		}

		if emailNotifier != nil {
			if err := emailNotifier.Test(); err != nil {
				emailStatus = fmt.Sprintf("发送失败: %v", err)
				logger.Error("邮件测试通知发送失败: %v", err)
			} else {
				emailStatus = "发送成功"
				logger.Info("邮件测试通知发送成功")
			}
		} else {
			emailStatus = "通知器未初始化"
		}
	} else {
		emailStatus = "未启用"
	}

	if s.config.Telegram.Enabled {
		hasAnyEnabled = true
		// 找到Telegram通知器并测试
		var telegramNotifier Notifier
		for _, notifier := range s.notification.GetNotifiers() {
			if notifier.GetType() == "telegram" && notifier.IsEnabled() {
				telegramNotifier = notifier
				break
			}
		}

		if telegramNotifier != nil {
			if err := telegramNotifier.Test(); err != nil {
				telegramStatus = fmt.Sprintf("发送失败: %v", err)
				logger.Error("Telegram测试通知发送失败: %v", err)
			} else {
				telegramStatus = "发送成功"
				logger.Info("Telegram测试通知发送成功")
			}
		} else {
			telegramStatus = "通知器未初始化"
		}
	} else {
		telegramStatus = "未启用"
	}

	// 如果没有启用任何通知方式
	if !hasAnyEnabled {
		s.writeJSON(w, map[string]interface{}{
			"status":          "warning",
			"message":         "未启用任何通知方式，请先在设置中配置并启用邮件或Telegram通知",
			"email_status":    emailStatus,
			"telegram_status": telegramStatus,
			"timestamp":       time.Now(),
		})
		return
	}

	// 判断整体状态
	overallStatus := "success"
	message := "测试通知发送完成"

	emailFailed := s.config.SMTP.Enabled && emailStatus != "发送成功"
	telegramFailed := s.config.Telegram.Enabled && telegramStatus != "发送成功"

	if emailFailed && telegramFailed {
		overallStatus = "error"
		message = "所有通知方式发送失败"
	} else if emailFailed || telegramFailed {
		overallStatus = "warning"
		message = "部分通知方式发送失败"
	}

	response := map[string]interface{}{
		"status":          overallStatus,
		"message":         message,
		"email_status":    emailStatus,
		"telegram_status": telegramStatus,
		"timestamp":       time.Now(),
	}

	s.writeJSON(w, response)
}

// handleDomainAdd 添加单个域名
func (s *Server) handleDomainAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Domain string `json:"domain"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	domain := strings.TrimSpace(request.Domain)
	domain = strings.ToLower(domain)

	if domain == "" {
		s.writeError(w, "域名不能为空", http.StatusBadRequest)
		return
	}

	// 验证域名格式
	if err := s.monitor.GetChecker().ValidateDomain(domain); err != nil {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "域名格式无效: " + err.Error(),
		})
		return
	}

	// 检查后缀是否支持
	tld := config.FindBestTLD(domain)
	if tld == "" {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "该后缀目前不支持进行监控",
		})
		return
	}

	// 检查域名是否已存在
	existingDomains, err := storage.ListDomains(false)
	if err != nil {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "检查域名失败: " + err.Error(),
		})
		return
	}

	for _, existing := range existingDomains {
		if strings.EqualFold(existing.Name, domain) {
			s.writeJSON(w, map[string]interface{}{
				"status":  "error",
				"message": "域名已存在",
			})
			return
		}
	}

	// 添加到监控列表
	if err := s.addDomainToConfig(domain); err != nil {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "添加域名失败: " + err.Error(),
		})
		return
	}

	// 重新加载域名列表
	if err := s.monitor.LoadDomains(); err != nil {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "重新加载域名失败: " + err.Error(),
		})
		return
	}

	// 如果监控未运行，尝试启动监控
	if !s.monitor.IsRunning() {
		if err := s.monitor.Start(); err != nil {
			logger.Warn("添加域名后自动启动监控失败: %v", err)
		} else {
			logger.Info("添加域名后自动启动监控成功")
		}
	}

	// 先创建一个占位记录，让前端能立即显示
	placeholderInfo := &core.DomainInfo{
		Name:        domain,
		Status:      core.StatusUnknown,
		LastChecked: time.Now(),
		QueryMethod: "checking",
	}

	// 添加域名到监控器（会立即启动worker并开始查询）
	if err := s.monitor.AddDomain(domain, true); err != nil {
		logger.Warn("添加域名到监控器失败: %v", err)
	} else {
		logger.Info("域名 %s 已添加到监控器", domain)
	}

	// 返回占位信息，让前端能立即显示
	s.writeJSON(w, map[string]interface{}{
		"status":  "success",
		"message": "Domain added successfully",
		"domain":  domain,
		"info":    placeholderInfo,
	})
}

// handleDomainBatchAdd 批量添加域名
func (s *Server) handleDomainBatchAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 速率限制检查
	clientIP := r.RemoteAddr
	if !s.batchLimiter.Allow(clientIP) {
		s.writeError(w, "批量添加过于频繁，请1分钟后再试", http.StatusTooManyRequests)
		return
	}

	var request struct {
		Domains []string `json:"domains"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(request.Domains) == 0 {
		s.writeError(w, "No domains provided", http.StatusBadRequest)
		return
	}

	// 限制批量添加数量
	if len(request.Domains) > 1000 {
		s.writeError(w, "一次最多添加1000个域名", http.StatusBadRequest)
		return
	}

	validDomains := []string{}
	invalidDomains := []string{}
	unsupportedDomains := []string{}

	// 获取已有域名列表
	existingDomainList, err := storage.ListDomains(false)
	if err != nil {
		s.writeError(w, "获取已有域名列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	existingMap := make(map[string]bool)
	for _, d := range existingDomainList {
		existingMap[strings.ToLower(d.Name)] = true
	}

	// 验证所有域名
	for _, domain := range request.Domains {
		domain = strings.TrimSpace(domain)
		domain = strings.ToLower(domain)

		if domain == "" {
			continue
		}

		// 检查域名格式
		if err := s.monitor.GetChecker().ValidateDomain(domain); err != nil {
			invalidDomains = append(invalidDomains, domain)
			continue
		}

		// 检查后缀是否支持
		tld := config.FindBestTLD(domain)
		if tld == "" {
			unsupportedDomains = append(unsupportedDomains, domain)
			continue
		}

		// 检查是否已存在
		if existingMap[domain] {
			continue
		}

		validDomains = append(validDomains, domain)
		existingMap[domain] = true
	}

	// 批量添加有效域名
	addedCount := 0
	for _, domain := range validDomains {
		if err := s.addDomainToConfig(domain); err == nil {
			addedCount++
		}
	}

	// 重新加载域名列表
	if addedCount > 0 {
		s.monitor.LoadDomains()

		// 如果监控未运行，尝试启动监控
		if !s.monitor.IsRunning() {
			if err := s.monitor.Start(); err != nil {
				logger.Warn("批量添加域名后自动启动监控失败: %v", err)
			} else {
				logger.Info("批量添加域名后自动启动监控成功")
			}
		}

		// 添加所有域名到监控器（会立即为每个域名启动worker并开始查询）
		for _, domain := range validDomains {
			if err := s.monitor.AddDomain(domain, true); err != nil {
				logger.Warn("添加域名 %s 到监控器失败: %v", domain, err)
			} else {
				logger.Info("域名 %s 已添加到监控器", domain)
			}
		}
	}

	response := map[string]interface{}{
		"status":              "success",
		"added_count":         addedCount,
		"invalid_count":       len(invalidDomains),
		"invalid_domains":     invalidDomains,
		"unsupported_count":   len(unsupportedDomains),
		"unsupported_domains": unsupportedDomains,
	}

	s.writeJSON(w, response)
}

// handleDomainRemove 删除域名
func (s *Server) handleDomainRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取域名
	path := strings.TrimPrefix(r.URL.Path, "/api/domain/remove/")
	domain := strings.TrimSuffix(path, "/")

	if domain == "" {
		s.writeError(w, "域名不能为空", http.StatusBadRequest)
		return
	}

	// 从配置中删除域名（包括domain_results）
	if err := s.removeDomainFromConfig(domain); err != nil {
		s.writeError(w, "删除域名失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 停止该域名的worker
	s.monitor.RemoveDomain(domain)
	logger.Info("域名 %s 已从监控器移除", domain)

	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Domain removed successfully",
		"domain":  domain,
	})
}

// handleChangePassword 处理修改密码请求
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodPost {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 解析请求
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证当前密码
	if !s.auth.ValidatePassword(req.CurrentPassword) {
		s.writeError(w, "当前密码错误", http.StatusBadRequest)
		return
	}

	// 验证新密码
	if len(req.NewPassword) < 6 {
		s.writeError(w, "新密码长度至少6位", http.StatusBadRequest)
		return
	}

	// 更新密码
	if err := s.auth.UpdatePassword(req.NewPassword); err != nil {
		log.Printf("更新密码失败: %v", err)
		s.writeError(w, "更新密码失败", http.StatusInternalServerError)
		return
	}

	// 持久化到数据库，便于重启后生效
	if err := storage.UpsertSettings(map[string]string{
		"server_password": req.NewPassword,
	}); err != nil {
		log.Printf("保存密码到数据库失败: %v", err)
	}

	// 同步内存配置
	s.config.Server.Password = req.NewPassword

	// 返回成功响应
	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "密码修改成功",
	})
}

// handleUpdateUsername 处理更新用户名请求
func (s *Server) handleUpdateUsername(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodPost {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 解析请求
	var req struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证用户名
	if len(req.Username) < 3 {
		s.writeError(w, "用户名长度至少3位", http.StatusBadRequest)
		return
	}

	// 更新用户名
	if err := s.auth.UpdateUsername(req.Username); err != nil {
		log.Printf("更新用户名失败: %v", err)
		s.writeError(w, "更新用户名失败", http.StatusInternalServerError)
		return
	}

	// 持久化到数据库
	if err := storage.UpsertSettings(map[string]string{
		"server_username": req.Username,
	}); err != nil {
		log.Printf("保存用户名到数据库失败: %v", err)
	}

	// 同步内存配置
	s.config.Server.Username = req.Username

	// 返回成功响应
	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "用户名更新成功",
	})
}

// handleSmtpSettings 处理SMTP设置
func (s *Server) handleSmtpSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		From     string `json:"from"`
		To       string `json:"to"`
		Enabled  bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 将设置保存到数据库
	if err := storage.UpsertSettings(map[string]string{
		"smtp_host":    req.Host,
		"smtp_port":    fmt.Sprintf("%d", req.Port),
		"smtp_user":    req.User,
		"smtp_pass":    req.Password,
		"smtp_from":    req.From,
		"smtp_to":      req.To,
		"smtp_enabled": fmt.Sprintf("%t", req.Enabled),
	}); err != nil {
		log.Printf("保存SMTP设置到数据库失败: %v", err)
		s.writeError(w, "保存设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新当前配置
	s.config.SMTP.Host = req.Host
	s.config.SMTP.Port = req.Port
	s.config.SMTP.User = req.User
	s.config.SMTP.Password = req.Password
	s.config.SMTP.From = req.From
	s.config.SMTP.To = req.To
	s.config.SMTP.Enabled = req.Enabled

	// 更新通知器配置
	if err := s.notification.UpdateEmailConfig(s.config.SMTP); err != nil {
		logger.Error("更新邮件通知器配置失败: %v", err)
	} else {
		logger.Info("已更新邮件通知器配置，启用状态: %v", req.Enabled)
	}

	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "SMTP设置保存成功",
	})
}

// handleTelegramSettings 处理Telegram设置
func (s *Server) handleTelegramSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BotToken string `json:"bot_token"`
		ChatID   string `json:"chat_id"`
		Enabled  bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 将设置保存到数据库
	if err := storage.UpsertSettings(map[string]string{
		"telegram_bot_token": req.BotToken,
		"telegram_chat_id":   req.ChatID,
		"telegram_enabled":   fmt.Sprintf("%t", req.Enabled),
	}); err != nil {
		log.Printf("保存Telegram设置到数据库失败: %v", err)
		s.writeError(w, "保存设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新当前配置
	s.config.Telegram.BotToken = req.BotToken
	s.config.Telegram.ChatID = req.ChatID
	s.config.Telegram.Enabled = req.Enabled

	// 更新通知器配置
	if err := s.notification.UpdateTelegramConfig(s.config.Telegram); err != nil {
		logger.Error("更新Telegram通知器配置失败: %v", err)
	} else {
		logger.Info("已更新Telegram通知器配置，启用状态: %v", req.Enabled)
	}

	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Telegram设置保存成功",
	})
}

// handleTestEmail 测试邮件发送
func (s *Server) handleTestEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "不允许的请求方法",
		})
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "未授权",
		})
		return
	}

	// 检查是否启用
	if !s.config.SMTP.Enabled {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "邮件通知未启用，请先在设置中启用并配置SMTP",
		})
		return
	}

	// 直接测试邮件通知器
	var emailNotifier Notifier
	for _, notifier := range s.notification.GetNotifiers() {
		if notifier.GetType() == "email" {
			emailNotifier = notifier
			break
		}
	}

	if emailNotifier == nil {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "未找到邮件通知器",
		})
		return
	}

	// 执行测试
	if err := emailNotifier.Test(); err != nil {
		logger.Error("测试邮件发送失败: %v", err)
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("测试邮件发送失败: %v", err),
		})
		return
	}

	logger.Info("测试邮件发送成功")
	s.writeJSON(w, map[string]interface{}{
		"status":  "success",
		"message": "测试邮件发送成功，请检查您的邮箱",
	})
}

// handleTestTelegram 测试Telegram发送
func (s *Server) handleTestTelegram(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "不允许的请求方法",
		})
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "未授权",
		})
		return
	}

	// 检查是否启用
	if !s.config.Telegram.Enabled {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "Telegram通知未启用，请先在设置中启用并配置Telegram Bot",
		})
		return
	}

	// 直接测试Telegram通知器
	var telegramNotifier Notifier
	for _, notifier := range s.notification.GetNotifiers() {
		if notifier.GetType() == "telegram" {
			telegramNotifier = notifier
			break
		}
	}

	if telegramNotifier == nil {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "未找到Telegram通知器",
		})
		return
	}

	// 执行测试
	if err := telegramNotifier.Test(); err != nil {
		logger.Error("测试Telegram发送失败: %v", err)
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("测试Telegram发送失败: %v", err),
		})
		return
	}

	logger.Info("测试Telegram发送成功")
	s.writeJSON(w, map[string]interface{}{
		"status":  "success",
		"message": "测试Telegram消息发送成功，请检查您的Telegram",
	})
}

// handleGetSettings 获取当前设置
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 返回当前设置（包含所有配置信息）
	settings := map[string]interface{}{
		"smtp": map[string]interface{}{
			"host":     s.config.SMTP.Host,
			"port":     s.config.SMTP.Port,
			"user":     s.config.SMTP.User,
			"password": s.config.SMTP.Password, // 返回密码
			"from":     s.config.SMTP.From,
			"to":       s.config.SMTP.To,
			"enabled":  s.config.SMTP.Enabled,
		},
		"telegram": map[string]interface{}{
			"bot_token": s.config.Telegram.BotToken, // 返回bot token
			"chat_id":   s.config.Telegram.ChatID,
			"enabled":   s.config.Telegram.Enabled,
		},
		"monitor": map[string]interface{}{
			"check_interval":   int(s.config.Monitor.CheckInterval.Seconds()),
			"concurrent_limit": s.config.Monitor.ConcurrentLimit,
			"timeout":          int(s.config.Monitor.Timeout.Seconds()),
		},
		"username": s.config.Server.Username,
	}

	s.writeJSON(w, settings)
}

// handleMonitorSettings 处理监控参数设置
func (s *Server) handleMonitorSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CheckInterval   int `json:"check_interval"`   // 检查间隔（秒）
		ConcurrentLimit int `json:"concurrent_limit"` // 并发限制
		Timeout         int `json:"timeout"`          // 超时时间（秒）
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证参数
	if req.CheckInterval < 5 {
		s.writeError(w, "检查间隔不能小于5秒", http.StatusBadRequest)
		return
	}
	if req.ConcurrentLimit <= 0 || req.ConcurrentLimit > 1000 {
		s.writeError(w, "并发限制必须在1-1000之间", http.StatusBadRequest)
		return
	}
	if req.Timeout <= 0 || req.Timeout > 120 {
		s.writeError(w, "超时时间必须在1-120秒之间", http.StatusBadRequest)
		return
	}

	// 将设置保存到数据库
	if err := storage.UpsertSettings(map[string]string{
		"monitor_check_interval":   fmt.Sprintf("%d", req.CheckInterval),
		"monitor_concurrent_limit": fmt.Sprintf("%d", req.ConcurrentLimit),
		"monitor_timeout":          fmt.Sprintf("%d", req.Timeout),
	}); err != nil {
		log.Printf("保存监控设置到数据库失败: %v", err)
		s.writeError(w, "保存设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 热重载：更新当前配置
	s.config.Monitor.CheckInterval = time.Duration(req.CheckInterval) * time.Second
	s.config.Monitor.ConcurrentLimit = req.ConcurrentLimit
	s.config.Monitor.Timeout = time.Duration(req.Timeout) * time.Second

	// 热重载：更新checker的配置
	if s.monitor.GetChecker() != nil {
		s.monitor.GetChecker().UpdateConfig(s.config)
	}

	// 热重载：通过UpdateConfig统一更新，它内部会处理并发限制
	s.monitor.UpdateConfig(s.config)

	logger.Info("监控参数已热重载: 间隔=%ds, 并发=%d, 超时=%ds",
		req.CheckInterval, req.ConcurrentLimit, req.Timeout)

	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "监控参数保存成功并已热重载",
	})
}

// handleDomainWhoisRaw 获取域名的原始WHOIS数据
func (s *Server) handleDomainWhoisRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取域名
	path := strings.TrimPrefix(r.URL.Path, "/api/domain/whois-raw/")
	domain := strings.TrimSuffix(path, "/")

	if domain == "" {
		s.writeError(w, "域名不能为空", http.StatusBadRequest)
		return
	}

	logger.Info("获取域名 %s 的原始WHOIS数据（从数据库）", domain)

	// 从数据库读取WHOIS原始数据
	result, err := storage.GetDomainResult(domain)
	if err != nil {
		s.writeError(w, "获取域名信息失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if result == nil || result.WhoisRaw == "" {
		s.writeError(w, "WHOIS data not found, please wait for the query to complete", http.StatusNotFound)
		return
	}

	s.writeJSON(w, map[string]interface{}{
		"domain":    domain,
		"whois_raw": result.WhoisRaw,
		"timestamp": result.LastChecked.Format("2006-01-02 15:04:05"),
	})
}

// filterDomains 过滤域名列表
func (s *Server) filterDomains(domains []*core.DomainInfo, searchTerm, statusFilter string) []*core.DomainInfo {
	if searchTerm == "" && statusFilter == "" {
		return domains
	}

	var filtered []*core.DomainInfo
	searchLower := strings.ToLower(searchTerm)

	for _, domain := range domains {
		// 检查搜索条件
		matchesSearch := searchTerm == "" || strings.Contains(strings.ToLower(domain.Name), searchLower)

		// 检查状态过滤条件
		matchesStatus := statusFilter == "" || string(domain.Status) == statusFilter

		// 调试：记录未匹配的域名
		if statusFilter != "" && !matchesStatus && matchesSearch {
			logger.Debug("域名 %s 状态为 %s，不匹配筛选条件 %s", domain.Name, domain.Status, statusFilter)
		}

		if matchesSearch && matchesStatus {
			filtered = append(filtered, domain)
		}
	}

	return filtered
}

// handleCleanOrphanedData 清理孤立数据处理器
func (s *Server) handleCleanOrphanedData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("收到清理孤立数据请求")

	// 执行清理
	if err := storage.CleanOrphanedData(); err != nil {
		logger.Error("清理孤立数据失败: %v", err)
		s.writeError(w, fmt.Sprintf("清理失败: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]interface{}{
		"status":  "success",
		"message": "孤立数据清理完成",
	})
}

// handleCheckUpdate 检查更新处理器
func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.writeError(w, "不允许的请求方法", http.StatusMethodNotAllowed)
		return
	}

	currentVersion := AppVersion

	// 获取最新版本信息
	resp, err := http.Get("https://spatioweb.com/puff/version")
	if err != nil {
		logger.Warn("检查更新失败: %v", err)
		s.writeJSON(w, map[string]interface{}{
			"error":          "无法检查更新",
			"currentVersion": currentVersion,
		})
		return
	}
	defer resp.Body.Close()

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		logger.Warn("解析更新信息失败: %v", err)
		s.writeJSON(w, map[string]interface{}{
			"error":          "解析更新信息失败",
			"currentVersion": currentVersion,
		})
		return
	}

	s.writeJSON(w, map[string]interface{}{
		"currentVersion":  currentVersion,
		"latestVersion":   release.TagName,
		"publishedAt":     release.PublishedAt,
		"updateAvailable": release.TagName != currentVersion,
		"announcement":    release.Body,
	})
}
