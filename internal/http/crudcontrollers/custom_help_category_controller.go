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

type CustomHelpCategoryController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewCustomHelpCategoryController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *CustomHelpCategoryController {
	return &CustomHelpCategoryController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *CustomHelpCategoryController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "custom_help_category/:id", e.getCustomHelpCategory, nil),
		routes.RegisterRoute(http.MethodGet, "custom_help_categories", e.listCustomHelpCategories, nil),
		routes.RegisterRoute(http.MethodGet, "custom_help_categories/count", e.getCustomHelpCategoriesCount, nil),
		routes.RegisterRoute(http.MethodPut, "custom_help_category", e.createCustomHelpCategory, nil),
		routes.RegisterRoute(http.MethodDelete, "custom_help_category/:id", e.deleteCustomHelpCategory, nil),
		routes.RegisterRoute(http.MethodPatch, "custom_help_category/:id", e.updateCustomHelpCategory, nil),
		routes.RegisterRoute(http.MethodPost, "custom_help_categories/bulk", e.getCustomHelpCategoriesBulk, nil),
	}
}

// listCustomHelpCategories godoc
// @Id listCustomHelpCategories
// @Summary Lists CustomHelpCategories
// @Accept json
// @Produce json
// @Tags CustomHelpCategory
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomHelpCategory
// @Failure 500 {string} string "Bad query request"
// @Router /custom_help_categories [get]
func (e *CustomHelpCategoryController) listCustomHelpCategories(c echo.Context) error {
	var results []models.CustomHelpCategory
	err := e.db.QueryContext(models.CustomHelpCategory{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomHelpCategory godoc
// @Id getCustomHelpCategory
// @Summary Gets CustomHelpCategory
// @Accept json
// @Produce json
// @Tags CustomHelpCategory
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomHelpCategory
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /custom_help_category/{id} [get]
func (e *CustomHelpCategoryController) getCustomHelpCategory(c echo.Context) error {
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
	var result models.CustomHelpCategory
	query := e.db.QueryContext(models.CustomHelpCategory{}, c)
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

// updateCustomHelpCategory godoc
// @Id updateCustomHelpCategory
// @Summary Updates CustomHelpCategory
// @Accept json
// @Produce json
// @Tags CustomHelpCategory
// @Param id path int true "Id"
// @Param custom_help_category body models.CustomHelpCategory true "CustomHelpCategory"
// @Success 200 {array} models.CustomHelpCategory
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /custom_help_category/{id} [patch]
func (e *CustomHelpCategoryController) updateCustomHelpCategory(c echo.Context) error {
	request := new(models.CustomHelpCategory)
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
	var result models.CustomHelpCategory
	query := e.db.QueryContext(models.CustomHelpCategory{}, c)
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
		event := fmt.Sprintf("Updated [CustomHelpCategory] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createCustomHelpCategory godoc
// @Id createCustomHelpCategory
// @Summary Creates CustomHelpCategory
// @Accept json
// @Produce json
// @Param custom_help_category body models.CustomHelpCategory true "CustomHelpCategory"
// @Tags CustomHelpCategory
// @Success 200 {array} models.CustomHelpCategory
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /custom_help_category [put]
func (e *CustomHelpCategoryController) createCustomHelpCategory(c echo.Context) error {
	customHelpCategory := new(models.CustomHelpCategory)
	if err := c.Bind(customHelpCategory); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.CustomHelpCategory{}, c).Model(&models.CustomHelpCategory{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&customHelpCategory).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.CustomHelpCategory{}, customHelpCategory)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [CustomHelpCategory] [%v] data [%v]", customHelpCategory.ID, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, customHelpCategory)
}

// deleteCustomHelpCategory godoc
// @Id deleteCustomHelpCategory
// @Summary Deletes CustomHelpCategory
// @Accept json
// @Produce json
// @Tags CustomHelpCategory
// @Param id path int true "id"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /custom_help_category/{id} [delete]
func (e *CustomHelpCategoryController) deleteCustomHelpCategory(c echo.Context) error {
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
	var result models.CustomHelpCategory
	query := e.db.QueryContext(models.CustomHelpCategory{}, c)
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
		event := fmt.Sprintf("Deleted [CustomHelpCategory] [%v] keys [%v]", result.ID, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getCustomHelpCategoriesBulk godoc
// @Id getCustomHelpCategoriesBulk
// @Summary Gets CustomHelpCategories in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags CustomHelpCategory
// @Success 200 {array} models.CustomHelpCategory
// @Failure 500 {string} string "Bad query request"
// @Router /custom_help_categories/bulk [post]
func (e *CustomHelpCategoryController) getCustomHelpCategoriesBulk(c echo.Context) error {
	var results []models.CustomHelpCategory

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

	err := e.db.QueryContext(models.CustomHelpCategory{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomHelpCategoriesCount godoc
// @Id getCustomHelpCategoriesCount
// @Summary Counts CustomHelpCategories
// @Accept json
// @Produce json
// @Tags CustomHelpCategory
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomHelpCategory
// @Failure 500 {string} string "Bad query request"
// @Router /custom_help_categories/count [get]
func (e *CustomHelpCategoryController) getCustomHelpCategoriesCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.CustomHelpCategory{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}