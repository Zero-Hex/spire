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

type CustomPetClassGrowthController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewCustomPetClassGrowthController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *CustomPetClassGrowthController {
	return &CustomPetClassGrowthController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *CustomPetClassGrowthController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "custom_pet_class_growth/:classId", e.getCustomPetClassGrowth, nil),
		routes.RegisterRoute(http.MethodGet, "custom_pet_class_growths", e.listCustomPetClassGrowths, nil),
		routes.RegisterRoute(http.MethodGet, "custom_pet_class_growths/count", e.getCustomPetClassGrowthsCount, nil),
		routes.RegisterRoute(http.MethodPut, "custom_pet_class_growth", e.createCustomPetClassGrowth, nil),
		routes.RegisterRoute(http.MethodDelete, "custom_pet_class_growth/:classId", e.deleteCustomPetClassGrowth, nil),
		routes.RegisterRoute(http.MethodPatch, "custom_pet_class_growth/:classId", e.updateCustomPetClassGrowth, nil),
		routes.RegisterRoute(http.MethodPost, "custom_pet_class_growths/bulk", e.getCustomPetClassGrowthsBulk, nil),
	}
}

// listCustomPetClassGrowths godoc
// @Id listCustomPetClassGrowths
// @Summary Lists CustomPetClassGrowths
// @Accept json
// @Produce json
// @Tags CustomPetClassGrowth
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetClassGrowth
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_class_growths [get]
func (e *CustomPetClassGrowthController) listCustomPetClassGrowths(c echo.Context) error {
	var results []models.CustomPetClassGrowth
	err := e.db.QueryContext(models.CustomPetClassGrowth{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomPetClassGrowth godoc
// @Id getCustomPetClassGrowth
// @Summary Gets CustomPetClassGrowth
// @Accept json
// @Produce json
// @Tags CustomPetClassGrowth
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetClassGrowth
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_class_growth/{id} [get]
func (e *CustomPetClassGrowthController) getCustomPetClassGrowth(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	classId, err := strconv.Atoi(c.Param("classId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [ClassId]"})
	}
	params = append(params, classId)
	keys = append(keys, "class_id = ?")

	// query builder
	var result models.CustomPetClassGrowth
	query := e.db.QueryContext(models.CustomPetClassGrowth{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	// couldn't find entity
	if result.ClassId == 0 {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Cannot find entity"})
	}

	return c.JSON(http.StatusOK, result)
}

// updateCustomPetClassGrowth godoc
// @Id updateCustomPetClassGrowth
// @Summary Updates CustomPetClassGrowth
// @Accept json
// @Produce json
// @Tags CustomPetClassGrowth
// @Param id path int true "Id"
// @Param custom_pet_class_growth body models.CustomPetClassGrowth true "CustomPetClassGrowth"
// @Success 200 {array} models.CustomPetClassGrowth
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /custom_pet_class_growth/{id} [patch]
func (e *CustomPetClassGrowthController) updateCustomPetClassGrowth(c echo.Context) error {
	request := new(models.CustomPetClassGrowth)
	if err := c.Bind(request); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	var params []interface{}
	var keys []string

	// primary key param
	classId, err := strconv.Atoi(c.Param("classId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [ClassId]"})
	}
	params = append(params, classId)
	keys = append(keys, "class_id = ?")

	// query builder
	var result models.CustomPetClassGrowth
	query := e.db.QueryContext(models.CustomPetClassGrowth{}, c)
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
		event := fmt.Sprintf("Updated [CustomPetClassGrowth] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createCustomPetClassGrowth godoc
// @Id createCustomPetClassGrowth
// @Summary Creates CustomPetClassGrowth
// @Accept json
// @Produce json
// @Param custom_pet_class_growth body models.CustomPetClassGrowth true "CustomPetClassGrowth"
// @Tags CustomPetClassGrowth
// @Success 200 {array} models.CustomPetClassGrowth
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /custom_pet_class_growth [put]
func (e *CustomPetClassGrowthController) createCustomPetClassGrowth(c echo.Context) error {
	customPetClassGrowth := new(models.CustomPetClassGrowth)
	if err := c.Bind(customPetClassGrowth); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.CustomPetClassGrowth{}, c).Model(&models.CustomPetClassGrowth{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&customPetClassGrowth).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.CustomPetClassGrowth{}, customPetClassGrowth)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [CustomPetClassGrowth] [%v] data [%v]", customPetClassGrowth.ClassId, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, customPetClassGrowth)
}

// deleteCustomPetClassGrowth godoc
// @Id deleteCustomPetClassGrowth
// @Summary Deletes CustomPetClassGrowth
// @Accept json
// @Produce json
// @Tags CustomPetClassGrowth
// @Param id path int true "classId"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /custom_pet_class_growth/{id} [delete]
func (e *CustomPetClassGrowthController) deleteCustomPetClassGrowth(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	classId, err := strconv.Atoi(c.Param("classId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	params = append(params, classId)
	keys = append(keys, "class_id = ?")

	// query builder
	var result models.CustomPetClassGrowth
	query := e.db.QueryContext(models.CustomPetClassGrowth{}, c)
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
		event := fmt.Sprintf("Deleted [CustomPetClassGrowth] [%v] keys [%v]", result.ClassId, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getCustomPetClassGrowthsBulk godoc
// @Id getCustomPetClassGrowthsBulk
// @Summary Gets CustomPetClassGrowths in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags CustomPetClassGrowth
// @Success 200 {array} models.CustomPetClassGrowth
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_class_growths/bulk [post]
func (e *CustomPetClassGrowthController) getCustomPetClassGrowthsBulk(c echo.Context) error {
	var results []models.CustomPetClassGrowth

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

	err := e.db.QueryContext(models.CustomPetClassGrowth{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomPetClassGrowthsCount godoc
// @Id getCustomPetClassGrowthsCount
// @Summary Counts CustomPetClassGrowths
// @Accept json
// @Produce json
// @Tags CustomPetClassGrowth
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetClassGrowth
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_class_growths/count [get]
func (e *CustomPetClassGrowthController) getCustomPetClassGrowthsCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.CustomPetClassGrowth{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}