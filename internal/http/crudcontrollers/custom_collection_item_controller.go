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

type CustomCollectionItemController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewCustomCollectionItemController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *CustomCollectionItemController {
	return &CustomCollectionItemController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *CustomCollectionItemController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "custom_collection_item/:id", e.getCustomCollectionItem, nil),
		routes.RegisterRoute(http.MethodGet, "custom_collection_items", e.listCustomCollectionItems, nil),
		routes.RegisterRoute(http.MethodGet, "custom_collection_items/count", e.getCustomCollectionItemsCount, nil),
		routes.RegisterRoute(http.MethodPut, "custom_collection_item", e.createCustomCollectionItem, nil),
		routes.RegisterRoute(http.MethodDelete, "custom_collection_item/:id", e.deleteCustomCollectionItem, nil),
		routes.RegisterRoute(http.MethodPatch, "custom_collection_item/:id", e.updateCustomCollectionItem, nil),
		routes.RegisterRoute(http.MethodPost, "custom_collection_items/bulk", e.getCustomCollectionItemsBulk, nil),
	}
}

// listCustomCollectionItems godoc
// @Id listCustomCollectionItems
// @Summary Lists CustomCollectionItems
// @Accept json
// @Produce json
// @Tags CustomCollectionItem
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomCollectionItem
// @Failure 500 {string} string "Bad query request"
// @Router /custom_collection_items [get]
func (e *CustomCollectionItemController) listCustomCollectionItems(c echo.Context) error {
	var results []models.CustomCollectionItem
	err := e.db.QueryContext(models.CustomCollectionItem{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomCollectionItem godoc
// @Id getCustomCollectionItem
// @Summary Gets CustomCollectionItem
// @Accept json
// @Produce json
// @Tags CustomCollectionItem
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomCollectionItem
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /custom_collection_item/{id} [get]
func (e *CustomCollectionItemController) getCustomCollectionItem(c echo.Context) error {
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
	var result models.CustomCollectionItem
	query := e.db.QueryContext(models.CustomCollectionItem{}, c)
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

// updateCustomCollectionItem godoc
// @Id updateCustomCollectionItem
// @Summary Updates CustomCollectionItem
// @Accept json
// @Produce json
// @Tags CustomCollectionItem
// @Param id path int true "Id"
// @Param custom_collection_item body models.CustomCollectionItem true "CustomCollectionItem"
// @Success 200 {array} models.CustomCollectionItem
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /custom_collection_item/{id} [patch]
func (e *CustomCollectionItemController) updateCustomCollectionItem(c echo.Context) error {
	request := new(models.CustomCollectionItem)
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
	var result models.CustomCollectionItem
	query := e.db.QueryContext(models.CustomCollectionItem{}, c)
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
		event := fmt.Sprintf("Updated [CustomCollectionItem] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createCustomCollectionItem godoc
// @Id createCustomCollectionItem
// @Summary Creates CustomCollectionItem
// @Accept json
// @Produce json
// @Param custom_collection_item body models.CustomCollectionItem true "CustomCollectionItem"
// @Tags CustomCollectionItem
// @Success 200 {array} models.CustomCollectionItem
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /custom_collection_item [put]
func (e *CustomCollectionItemController) createCustomCollectionItem(c echo.Context) error {
	customCollectionItem := new(models.CustomCollectionItem)
	if err := c.Bind(customCollectionItem); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.CustomCollectionItem{}, c).Model(&models.CustomCollectionItem{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&customCollectionItem).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.CustomCollectionItem{}, customCollectionItem)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [CustomCollectionItem] [%v] data [%v]", customCollectionItem.ID, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, customCollectionItem)
}

// deleteCustomCollectionItem godoc
// @Id deleteCustomCollectionItem
// @Summary Deletes CustomCollectionItem
// @Accept json
// @Produce json
// @Tags CustomCollectionItem
// @Param id path int true "id"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /custom_collection_item/{id} [delete]
func (e *CustomCollectionItemController) deleteCustomCollectionItem(c echo.Context) error {
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
	var result models.CustomCollectionItem
	query := e.db.QueryContext(models.CustomCollectionItem{}, c)
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
		event := fmt.Sprintf("Deleted [CustomCollectionItem] [%v] keys [%v]", result.ID, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getCustomCollectionItemsBulk godoc
// @Id getCustomCollectionItemsBulk
// @Summary Gets CustomCollectionItems in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags CustomCollectionItem
// @Success 200 {array} models.CustomCollectionItem
// @Failure 500 {string} string "Bad query request"
// @Router /custom_collection_items/bulk [post]
func (e *CustomCollectionItemController) getCustomCollectionItemsBulk(c echo.Context) error {
	var results []models.CustomCollectionItem

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

	err := e.db.QueryContext(models.CustomCollectionItem{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomCollectionItemsCount godoc
// @Id getCustomCollectionItemsCount
// @Summary Counts CustomCollectionItems
// @Accept json
// @Produce json
// @Tags CustomCollectionItem
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomCollectionItem
// @Failure 500 {string} string "Bad query request"
// @Router /custom_collection_items/count [get]
func (e *CustomCollectionItemController) getCustomCollectionItemsCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.CustomCollectionItem{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}