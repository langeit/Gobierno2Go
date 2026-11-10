package models

type User struct {
	IDUsuario string `json:"id_usuario"`
	Login     string `json:"login"`
	Password  string `json:"-"`
	Estado    string `json:"estado"`
	Nombre    string `json:"nombre,omitempty"`
	Tipo      string `json:"tipo,omitempty"`
}

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	User     *User  `json:"user,omitempty"`
	Redirect string `json:"redirect,omitempty"`
}
