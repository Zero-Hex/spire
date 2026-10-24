package crudcontrollers

import (
	"fmt"
	"github.com/EQEmu/spire/internal/auditlog"
	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strconv"
	"strings"
)

type CustomPetSpecieController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewCustomPetSpecieController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *CustomPetSpecieController {
	return &CustomPetSpecieController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *CustomPetSpecieController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "custom_pet_specie/:id", e.getCustomPetSpecie, nil),
		routes.RegisterRoute(http.MethodGet, "custom_pet_species", e.listCustomPetSpecies, nil),
		routes.RegisterRoute(http.MethodGet, "custom_pet_species/count", e.getCustomPetSpeciesCount, nil),
		routes.RegisterRoute(http.MethodPut, "custom_pet_specie", e.createCustomPetSpecie, nil),
		routes.RegisterRoute(http.MethodDelete, "custom_pet_specie/:id", e.deleteCustomPetSpecie, nil),
		routes.RegisterRoute(http.MethodPatch, "custom_pet_specie/:id", e.updateCustomPetSpecie, nil),
		routes.RegisterRoute(http.MethodPost, "custom_pet_species/bulk", e.getCustomPetSpeciesBulk, nil),
	}
}

// listCustomPetSpecies godoc
// @Id listCustomPetSpecies
// @Summary Lists CustomPetSpecies
// @Accept json
// @Produce json
// @Tags CustomPetSpecie
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetSpecie
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_species [get]
func (e *CustomPetSpecieController) listCustomPetSpecies(c echo.Context) error {
	var results []models.CustomPetSpecie
	err := e.db.QueryContext(models.CustomPetSpecie{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomPetSpecie godoc
// @Id getCustomPetSpecie
// @Summary Gets CustomPetSpecie
// @Accept json
// @Produce json
// @Tags CustomPetSpecie
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetSpecie
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_specie/{id} [get]
func (e *CustomPetSpecieController) getCustomPetSpecie(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [Id]"})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomPetSpecie
	query := e.db.QueryContext(models.CustomPetSpecie{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	// couldn't find entity
	if result.ID == 0 {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Cannot find entity"})
	}

	return c.JSON(http.StatusOK, result)
}

// updateCustomPetSpecie godoc
// @Id updateCustomPetSpecie
// @Summary Updates CustomPetSpecie
// @Accept json
// @Produce json
// @Tags CustomPetSpecie
// @Param id path int true "Id"
// @Param custom_pet_specie body models.CustomPetSpecie true "CustomPetSpecie"
// @Success 200 {array} models.CustomPetSpecie
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /custom_pet_specie/{id} [patch]
func (e *CustomPetSpecieController) updateCustomPetSpecie(c echo.Context) error {
	request := new(models.CustomPetSpecie)
	if err := c.Bind(request); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [Id]"})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomPetSpecie
	query := e.db.QueryContext(models.CustomPetSpecie{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("Cannot find entity [%s]", err.Error())})
	}

	// save top-level using only changes
	diff := database.ResultDifference(result, request)
	err = query.Session(&gorm.Session{FullSaveAssociations: false}).Updates(diff).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("Error updating entity [%v]", err.Error())})
	}

	// log update event
	if e.db.GetSpireDb() != nil && len(diff) > 0 {
		// build ids
		var ids []string
		for i, _ := range keys {
			param := fmt.Sprintf("%v", params[i])
			ids = append(ids, fmt.Sprintf("%v", strings.ReplaceAll(keys[i], "?", param)))
		}
		// build fields updated
		var fieldsUpdated []string
		for k, v := range diff {
			fieldsUpdated = append(fieldsUpdated, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Updated [CustomPetSpecie] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createCustomPetSpecie godoc
// @Id createCustomPetSpecie
// @Summary Creates CustomPetSpecie
// @Accept json
// @Produce json
// @Param custom_pet_specie body models.CustomPetSpecie true "CustomPetSpecie"
// @Tags CustomPetSpecie
// @Success 200 {array} models.CustomPetSpecie
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /custom_pet_specie [put]
func (e *CustomPetSpecieController) createCustomPetSpecie(c echo.Context) error {
	customPetSpecie := new(models.CustomPetSpecie)
	if err := c.Bind(customPetSpecie); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.CustomPetSpecie{}, c).Model(&models.CustomPetSpecie{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&customPetSpecie).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.CustomPetSpecie{}, customPetSpecie)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [CustomPetSpecie] [%v] data [%v]", customPetSpecie.ID, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, customPetSpecie)
}

// deleteCustomPetSpecie godoc
// @Id deleteCustomPetSpecie
// @Summary Deletes CustomPetSpecie
// @Accept json
// @Produce json
// @Tags CustomPetSpecie
// @Param id path int true "id"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /custom_pet_specie/{id} [delete]
func (e *CustomPetSpecieController) deleteCustomPetSpecie(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomPetSpecie
	query := e.db.QueryContext(models.CustomPetSpecie{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	err = query.Limit(10000).Delete(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Error deleting entity"})
	}

	// log delete event
	if e.db.GetSpireDb() != nil {
		// build ids
		var ids []string
		for i, _ := range keys {
			param := fmt.Sprintf("%v", params[i])
			ids = append(ids, fmt.Sprintf("%v", strings.ReplaceAll(keys[i], "?", param)))
		}
		// record event
		event := fmt.Sprintf("Deleted [CustomPetSpecie] [%v] keys [%v]", result.ID, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getCustomPetSpeciesBulk godoc
// @Id getCustomPetSpeciesBulk
// @Summary Gets CustomPetSpecies in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags CustomPetSpecie
// @Success 200 {array} models.CustomPetSpecie
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_species/bulk [post]
func (e *CustomPetSpecieController) getCustomPetSpeciesBulk(c echo.Context) error {
	var results []models.CustomPetSpecie

	r := new(BulkFetchByIdsGetRequest)
	if err := c.Bind(r); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to bulk request: [%v]", err.Error())},
		)
	}

	if len(r.IDs) == 0 {
		return c.JSON(
			http.StatusOK,
			echo.Map{"error": fmt.Sprintf("Missing request field data 'ids'")},
		)
	}

	err := e.db.QueryContext(models.CustomPetSpecie{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomPetSpeciesCount godoc
// @Id getCustomPetSpeciesCount
// @Summary Counts CustomPetSpecies
// @Accept json
// @Produce json
// @Tags CustomPetSpecie
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetSpecie
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_species/count [get]
func (e *CustomPetSpecieController) getCustomPetSpeciesCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.CustomPetSpecie{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}