package repository

import (
	"errors"
	"testing"

	"Gobierno2Go/internal/config"
	"Gobierno2Go/internal/models"
)

func TestPaginaMateriales(t *testing.T) {
	p := PaginaMateriales(16, 0)
	if p.Actual != 1 || p.Total != 2 || len(p.Paginas) != 2 {
		t.Fatalf("pagina = %+v", p)
	}
	p = PaginaMateriales(0, 3)
	if p.Actual != 1 || p.Total != 1 {
		t.Fatalf("sin registros: %+v", p)
	}
}

func TestMantenedoresAdquisiciones(t *testing.T) {
	db, err := config.ConnectDB()
	if err != nil {
		t.Skip("sin base de datos:", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := NewAdquisicionesRepository(db)
	if err := repo.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureSchema(); err != nil {
		t.Fatal("el esquema debe poder aplicarse dos veces:", err)
	}

	const (
		descTipo = "ZZ Tipo mantenedor"
		descDom  = "ZZ Dominio mantenedor"
		nomMat   = "ZZ Material mantenedor"
		codAsig  = "ZZ-TEST"
		rut      = "ZZ111111K"
		rutNuevo = "ZZ222222K"
	)
	limpiar := func() {
		db.Exec(`DELETE FROM adu_materialmm WHERE nombre_material = $1`, nomMat)
		db.Exec(`DELETE FROM adu_proveedormm WHERE btrim(rut_proveedor) IN ($1, $2)`, rut, rutNuevo)
		db.Exec(`DELETE FROM adu_asignacion WHERE btrim(cod_asig) = $1`, codAsig)
		db.Exec(`DELETE FROM adu_dominio WHERE desc_dominio = $1`, descDom)
		db.Exec(`DELETE FROM adu_tipo_dominio WHERE desc_tipo_dom = $1`, descTipo)
	}
	limpiar()
	t.Cleanup(limpiar)

	if err := repo.CrearTipoDominio("  "); !errors.Is(err, ErrDatos) {
		t.Fatalf("descripción vacía: %v", err)
	}
	if err := repo.CrearTipoDominio(descTipo); err != nil {
		t.Fatal(err)
	}
	tipos, err := repo.TiposDominio()
	if err != nil {
		t.Fatal(err)
	}
	var tipoID int
	for _, tipo := range tipos {
		if tipo.Descripcion == descTipo {
			tipoID = tipo.Codigo
		}
	}
	if tipoID == 0 {
		t.Fatal("no apareció el tipo creado")
	}
	if err := repo.CrearDominio(tipoID, descDom); err != nil {
		t.Fatal(err)
	}
	if err := repo.EliminarTipoDominio(tipoID); !errors.Is(err, ErrRegistroEnUso) {
		t.Fatalf("un tipo con dominios no se elimina: %v", err)
	}

	materialesTipo, err := repo.DominiosPorTipo(models.CodTipoMaterial)
	if err != nil || len(materialesTipo) == 0 {
		t.Fatal("faltan dominios de tipo material", err)
	}
	if err := repo.CrearMaterial(models.Material{
		Nombre: nomMat, Tipo: materialesTipo[0].Codigo, StockCritico: 3,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CrearMaterial(models.Material{Nombre: nomMat, Tipo: tipoID, StockCritico: 1}); !errors.Is(err, ErrDatos) {
		t.Fatalf("un dominio que no es tipo material debe rechazarse: %v", err)
	}
	if err := repo.EliminarDominio(materialesTipo[0].Codigo); !errors.Is(err, ErrRegistroEnUso) {
		t.Fatalf("un dominio usado por un material no se elimina: %v", err)
	}

	regiones, err := repo.DominiosPorTipo(models.CodTipoRegion)
	if err != nil || len(regiones) == 0 {
		t.Fatal("faltan regiones", err)
	}
	p := models.Proveedor{
		Rut: rut, Nombre: "Proveedor ZZ", Region: regiones[0].Codigo, Mail: "zz@ejemplo.cl",
	}
	if err := repo.CrearProveedor(p); err != nil {
		t.Fatal(err)
	}
	if err := repo.CrearProveedor(p); !errors.Is(err, ErrDuplicado) {
		t.Fatalf("rut duplicado: %v", err)
	}
	p.Mail = "correo-malo"
	if err := repo.ActualizarProveedor(rut, p); !errors.Is(err, ErrDatos) {
		t.Fatalf("correo inválido: %v", err)
	}
	p.Mail = "zz@ejemplo.cl"
	p.Rut = rutNuevo
	if err := repo.ActualizarProveedor(rut, p); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Proveedor(rutNuevo); err != nil {
		t.Fatal(err)
	}

	if err := repo.CrearAsignacion(models.Asignacion{Codigo: codAsig, Descripcion: "Asignación de prueba"}); err != nil {
		t.Fatal(err)
	}
	lista, err := repo.AsignacionesMantenedor()
	if err != nil {
		t.Fatal(err)
	}
	var asigID int
	for _, a := range lista {
		if a.Codigo == codAsig {
			asigID = a.Correlativo
		}
	}
	if asigID == 0 {
		t.Fatal("no apareció la asignación creada")
	}
	if err := repo.EliminarAsignacion(asigID); err != nil {
		t.Fatal(err)
	}
}
