package dashboard

import (
	DB "SCTI/database"
	Erros "SCTI/erros"
	"html/template"
	"net/http"
)

type DashboardData struct {
	IsVerified           bool
	IsAdmin              bool
	Activities           []DB.Activity
	RegisteredActivities []DB.Activity
}

func GetDashboard(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("accessToken")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if cookie.Value == "-1" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	all_activities, _ := DB.GetAllActivities()
	registered_activities, _ := DB.GetUserActivities(cookie.Value)
	available_activities := RemoveRegisteredActivities(all_activities, registered_activities)
	email := DB.GetEmail(cookie.Value)
	standing := DB.GetStanding(email)

	admin := false
	admcookie, err := r.Cookie("Admin")
	if err == nil && admcookie.Value == cookie.Value {
		admin = true
	} else {
		http.SetCookie(w, &http.Cookie{
			Name:     "Admin",
			Value:    "",
			MaxAge:   -1,
			Secure:   false,
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
		})
	}

	data := DashboardData{
		IsVerified:           standing,
		IsAdmin:              admin,
		Activities:           available_activities,
		RegisteredActivities: registered_activities,
	}

	tmpl, err := template.ParseFiles("template/dashboard.gohtml")
	if err != nil {
		Erros.HttpError(w, "dashboard/handler", err)
		return
	}
	err = tmpl.ExecuteTemplate(w, "dashboard", data)
	if err != nil {
		Erros.HttpError(w, "dashboard/handler", err)
	}
}

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard", GetDashboard)
	mux.HandleFunc("GET /presenca", GetAttendance)
	mux.HandleFunc("POST /marcar-presenca", MarkAttendance)
	mux.HandleFunc("POST /desmarcar-presenca", UnmarkAttendance)
	mux.HandleFunc("POST /cadastrar", PostCadastros)
	mux.HandleFunc("POST /descadastrar", PostDescadastros)
	mux.HandleFunc("POST /send-verification-email", VerifyEmail)
	mux.HandleFunc("POST /set-admin", SetAdmin)
	mux.HandleFunc("POST /remover-admin", RemoveAdmin)
	mux.HandleFunc("POST /add_activity", PostActivity)
	mux.HandleFunc("POST /markpaid", PostValidateEmail)
	mux.HandleFunc("POST /qr-all", AllUsersSentQR)
	mux.HandleFunc("POST /qr-user", UserSentQR)
}
