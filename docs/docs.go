// Code generated by swaggo/swag. DO NOT EDIT.

package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/db/query": {
            "get": {
                "description": "execute query",
                "summary": "Execute query",
                "parameters": [
                    {
                        "type": "string",
                        "default": "select * from example limit 3",
                        "description": "sql query text",
                        "name": "q",
                        "in": "query",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/msg.QueryResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/msg.QueryResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/msg.QueryResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "msg.QueryData": {
            "type": "object",
            "properties": {
                "columns": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                },
                "rows": {
                    "type": "array",
                    "items": {
                        "type": "array",
                        "items": {}
                    }
                },
                "types": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                }
            }
        },
        "msg.QueryResponse": {
            "type": "object",
            "properties": {
                "data": {
                    "$ref": "#/definitions/msg.QueryData"
                },
                "elapse": {
                    "type": "string"
                },
                "reason": {
                    "type": "string"
                },
                "success": {
                    "type": "boolean"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{},
	Title:            "",
	Description:      "",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
