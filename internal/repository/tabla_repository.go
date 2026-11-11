package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"Gobierno2Go/internal/models"

	"github.com/lib/pq"
)

// ErrReferencia indica que un valor seleccionado ya no existe.
var ErrReferencia = errors.New("la referencia seleccionada no existe")

// TablaRepository ejecuta el CRUD de las tablas del menú Bases.
type TablaRepository struct {
	db *sql.DB
}

func NewTablaRepository(db *sql.DB) *TablaRepository {
	return &TablaRepository{db: db}
}

func columnas(campos []models.TablaCampo) []string {
	out := make([]string, 0, len(campos))
	for _, c := range campos {
		out = append(out, pq.QuoteIdentifier(c.Nombre))
	}
	return out
}

func (r *TablaRepository) consultar(def models.TablaDef, condicion string, args ...interface{}) ([]map[string]string, error) {
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s",
		strings.Join(columnas(def.Campos), ", "),
		pq.QuoteIdentifier(def.Tabla),
		condicion,
	)
	if len(def.Orden) > 0 {
		orden := make([]string, 0, len(def.Orden))
		for _, c := range def.Orden {
			orden = append(orden, pq.QuoteIdentifier(c))
		}
		query += " ORDER BY " + strings.Join(orden, ", ")
	}
	return r.consultarSQL(query, args...)
}

func (r *TablaRepository) consultarSQL(query string, args ...interface{}) ([]map[string]string, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]string
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		valores := make([]interface{}, len(cols))
		destinos := make([]interface{}, len(cols))
		for i := range valores {
			destinos[i] = &valores[i]
		}
		if err := rows.Scan(destinos...); err != nil {
			return nil, err
		}
		fila := make(map[string]string, len(cols))
		for i, c := range cols {
			fila[c] = valorTexto(valores[i])
		}
		out = append(out, fila)
	}
	return out, rows.Err()
}

func valorTexto(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case string:
		return x
	case time.Time:
		return x.Format("2006-01-02")
	case bool:
		return strconv.FormatBool(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

// normalizarPagina acota la página pedida al rango válido para el total indicado.
func normalizarPagina(total, porPagina, pagina int) (int, int) {
	if porPagina <= 0 {
		porPagina = 15
	}
	totalPaginas := (total + porPagina - 1) / porPagina
	if totalPaginas < 1 {
		totalPaginas = 1
	}
	if pagina < 1 {
		pagina = 1
	}
	if pagina > totalPaginas {
		pagina = totalPaginas
	}
	return pagina, totalPaginas
}

// ListaOpciones agrupa el filtro, la búsqueda, el orden y la página del listado.
type ListaOpciones struct {
	FiltroColumna string
	FiltroValor   string
	Busqueda      string
	OrdenColumna  string
	Ascendente    bool
	Pagina        int
	PorPagina     int
}

// Listar devuelve una página de filas vigentes. El total corresponde a todas
// las filas que cumplen el filtro y la búsqueda, y la página devuelta queda
// acotada al rango existente.
func (r *TablaRepository) Listar(def models.TablaDef, opciones ListaOpciones) ([]map[string]string, int, int, error) {
	condiciones := []string{"estado = 'V'"}
	var args []interface{}
	if opciones.FiltroColumna != "" {
		args = append(args, opciones.FiltroValor)
		condiciones = append(condiciones, fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(opciones.FiltroColumna), len(args)))
	}
	if busqueda := strings.TrimSpace(opciones.Busqueda); busqueda != "" {
		args = append(args, "%"+busqueda+"%")
		indice := len(args)
		var coincidencias []string
		for _, c := range def.Campos {
			columna := pq.QuoteIdentifier(def.Tabla) + "." + pq.QuoteIdentifier(c.Nombre)
			coincidencias = append(coincidencias, fmt.Sprintf("COALESCE(%s::text, '') ILIKE $%d", columna, indice))
			if c.Tipo == models.CampoReferencia && c.RefTabla != "" {
				coincidencias = append(coincidencias, fmt.Sprintf(
					"EXISTS (SELECT 1 FROM %s ref WHERE ref.%s = %s AND COALESCE(ref.%s::text, '') ILIKE $%d)",
					pq.QuoteIdentifier(c.RefTabla),
					pq.QuoteIdentifier(c.RefValor),
					columna,
					pq.QuoteIdentifier(c.RefEtiqueta),
					indice,
				))
			}
		}
		condiciones = append(condiciones, "("+strings.Join(coincidencias, " OR ")+")")
	}
	where := strings.Join(condiciones, " AND ")

	var total int
	if err := r.db.QueryRow(fmt.Sprintf(
		"SELECT COUNT(*) FROM %s WHERE %s",
		pq.QuoteIdentifier(def.Tabla), where,
	), args...).Scan(&total); err != nil {
		return nil, 0, 1, err
	}

	porPagina := opciones.PorPagina
	if porPagina <= 0 {
		porPagina = 15
	}
	pagina, _ := normalizarPagina(total, porPagina, opciones.Pagina)

	orden := ""
	if opciones.OrdenColumna != "" {
		if c, ok := def.Campo(opciones.OrdenColumna); ok {
			if c.Tipo == models.CampoReferencia && c.RefTabla != "" {
				orden = fmt.Sprintf(
					"(SELECT ref.%s FROM %s ref WHERE ref.%s = %s.%s)",
					pq.QuoteIdentifier(c.RefEtiqueta),
					pq.QuoteIdentifier(c.RefTabla),
					pq.QuoteIdentifier(c.RefValor),
					pq.QuoteIdentifier(def.Tabla),
					pq.QuoteIdentifier(c.Nombre),
				)
			} else {
				orden = pq.QuoteIdentifier(def.Tabla) + "." + pq.QuoteIdentifier(c.Nombre)
			}
			if opciones.Ascendente {
				orden += " ASC"
			} else {
				orden += " DESC"
			}
		}
	}
	if orden == "" && len(def.Orden) > 0 {
		partes := make([]string, 0, len(def.Orden))
		for _, c := range def.Orden {
			partes = append(partes, pq.QuoteIdentifier(def.Tabla)+"."+pq.QuoteIdentifier(c))
		}
		orden = strings.Join(partes, ", ") + " ASC"
	}

	args = append(args, porPagina, (pagina-1)*porPagina)
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s",
		strings.Join(columnas(def.Campos), ", "),
		pq.QuoteIdentifier(def.Tabla),
		where,
	)
	if orden != "" {
		query += " ORDER BY " + orden
	}
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	filas, err := r.consultarSQL(query, args...)
	if err != nil {
		return nil, 0, pagina, err
	}
	return filas, total, pagina, nil
}

// Fila devuelve un registro vigente por su clave.
func (r *TablaRepository) Fila(def models.TablaDef, clave map[string]string) (map[string]string, error) {
	condicion, args := condicionClave(def, clave)
	filas, err := r.consultar(def, condicion+" AND estado = 'V'", args...)
	if err != nil {
		return nil, err
	}
	if len(filas) == 0 {
		return nil, sql.ErrNoRows
	}
	return filas[0], nil
}

// EtiquetaRegistro devuelve el texto que identifica a un registro.
func (r *TablaRepository) EtiquetaRegistro(def models.TablaDef, clave map[string]string) (string, error) {
	columna := def.Etiqueta
	if columna == "" && len(def.Campos) > 0 {
		columna = def.Campos[0].Nombre
	}
	condicion, args := condicionClave(def, clave)
	var etiqueta interface{}
	err := r.db.QueryRow(fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s AND estado = 'V'",
		pq.QuoteIdentifier(columna), pq.QuoteIdentifier(def.Tabla), condicion,
	), args...).Scan(&etiqueta)
	if err != nil {
		return "", err
	}
	return valorTexto(etiqueta), nil
}

func condicionClave(def models.TablaDef, clave map[string]string) (string, []interface{}) {
	return condicionClaveDesde(def, clave, 0)
}

func condicionClaveDesde(def models.TablaDef, clave map[string]string, offset int) (string, []interface{}) {
	var partes []string
	var args []interface{}
	for i, c := range def.Clave() {
		partes = append(partes, fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(c.Nombre), offset+i+1))
		args = append(args, clave[c.Nombre])
	}
	return strings.Join(partes, " AND "), args
}

// Guardar inserta o actualiza un registro. Si editar es verdadero se actualiza
// el registro identificado por clave; si no, se inserta con los mismos valores.
func (r *TablaRepository) Guardar(def models.TablaDef, valores map[string]string, clave map[string]string, editar bool) error {
	if !editar {
		var cols []string
		var marcas []string
		var args []interface{}
		for _, c := range def.Campos {
			cols = append(cols, pq.QuoteIdentifier(c.Nombre))
			marcas = append(marcas, fmt.Sprintf("$%d", len(args)+1))
			args = append(args, valorParametro(c, valores[c.Nombre]))
		}
		_, err := r.db.Exec(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)",
			pq.QuoteIdentifier(def.Tabla), strings.Join(cols, ", "), strings.Join(marcas, ", "),
		), args...)
		return clasificarError(err)
	}

	var asignaciones []string
	var args []interface{}
	for _, c := range def.Campos {
		if c.Clave {
			continue
		}
		args = append(args, valorParametro(c, valores[c.Nombre]))
		asignaciones = append(asignaciones, fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(c.Nombre), len(args)))
	}
	if len(asignaciones) == 0 {
		if _, err := r.Fila(def, clave); err != nil {
			return err
		}
		return nil
	}
	condicion, argsClave := condicionClaveDesde(def, clave, len(args))
	args = append(args, argsClave...)
	res, err := r.db.Exec(fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s AND estado = 'V'",
		pq.QuoteIdentifier(def.Tabla), strings.Join(asignaciones, ", "), condicion,
	), args...)
	if err != nil {
		return clasificarError(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func valorParametro(c models.TablaCampo, v string) interface{} {
	switch c.Tipo {
	case models.CampoEntero:
		if v == "" {
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return v
		}
		return n
	case models.CampoDecimal:
		if v == "" {
			return nil
		}
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return v
		}
		return n
	case models.CampoFecha:
		if v == "" {
			return nil
		}
		return v
	case models.CampoBooleano:
		return v == "true"
	default:
		return v
	}
}

// Eliminar aplica borrado lógico si el registro no está siendo usado.
func (r *TablaRepository) Eliminar(def models.TablaDef, clave map[string]string) (string, error) {
	claveCampos := def.Clave()
	if len(claveCampos) > 0 {
		for _, uso := range def.UsadaPor {
			consulta := fmt.Sprintf(
				"SELECT EXISTS (SELECT 1 FROM %s WHERE %s = $1 AND estado = 'V')",
				pq.QuoteIdentifier(uso.Tabla), pq.QuoteIdentifier(uso.Campo),
			)
			var usado bool
			if err := r.db.QueryRow(consulta, clave[claveCampos[0].Nombre]).Scan(&usado); err != nil {
				return "", err
			}
			if usado {
				return uso.Detalle, ErrRegistroEnUso
			}
		}
	}

	condicion, args := condicionClave(def, clave)
	res, err := r.db.Exec(fmt.Sprintf(
		"UPDATE %s SET estado = 'E' WHERE %s AND estado = 'V'",
		pq.QuoteIdentifier(def.Tabla), condicion,
	), args...)
	if err != nil {
		return "", clasificarError(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", sql.ErrNoRows
	}
	return "", nil
}

// OpcionesReferencia carga las opciones vigentes de una tabla referenciada.
func (r *TablaRepository) OpcionesReferencia(c models.TablaCampo) ([]models.OpcionCampo, error) {
	valor := c.RefValor
	if valor == "" {
		valor = "codigo"
	}
	etiqueta := c.RefEtiqueta
	if etiqueta == "" {
		etiqueta = "descripcion"
	}
	query := fmt.Sprintf(
		"SELECT %s, %s FROM %s WHERE estado = 'V' ORDER BY %s",
		pq.QuoteIdentifier(valor),
		pq.QuoteIdentifier(etiqueta),
		pq.QuoteIdentifier(c.RefTabla),
		pq.QuoteIdentifier(etiqueta),
	)
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.OpcionCampo
	for rows.Next() {
		var v, e string
		if err := rows.Scan(&v, &e); err != nil {
			return nil, err
		}
		out = append(out, models.OpcionCampo{Valor: v, Etiqueta: v + " — " + e})
	}
	return out, rows.Err()
}

func clasificarError(err error) error {
	if err == nil {
		return nil
	}
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}
	switch pqErr.Code {
	case "23505":
		return ErrDuplicado
	case "23503":
		return ErrReferencia
	case "23514", "22001", "22P02", "22003", "22007", "22008":
		return ErrDatos
	default:
		return err
	}
}
