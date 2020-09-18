package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
	"github.com/hakobera/serverless-webrtc-signaling-server/common"
)

type RegisterCommand struct {
	Type     string `json:"type"`
	RoomID   string `json:"roomId"`
	ClientID string `json:"clientId"`
}

var db = common.NewDB(session.New(), aws.NewConfig())

func registerHandler(api common.ApiGatewayManagementAPI, db common.DB, connectionID, body string, now time.Time) error {
	cmd := RegisterCommand{}
	err := json.Unmarshal([]byte(body), &cmd)
	if err != nil {
		return err
	}

	connectionsTable := db.ConnectionsTable()
	roomsTable := db.RoomsTable()

	var room common.Room
	err = roomsTable.FindOne("roomId", cmd.RoomID, &room)
	if err != nil {
		if err.Error() == dynamo.ErrNotFound.Error() {
			room = common.Room{RoomID: cmd.RoomID, Clients: []common.Client{}, Created: now}
		} else {
			return err
		}
	}

	resultType := "accept"
	isExistClient := false
	if len(room.Clients) == 1 {
		isExistClient = true
	}

	if len(room.Clients) < 2 {
		client := common.Client{ConnectionID: connectionID, ClientID: cmd.ClientID, Joined: now}
		room.Clients = append(room.Clients, client)
		conn := common.Connection{ConnectionID: connectionID, RoomID: room.RoomID}
		err := db.TxPut(
			common.TableItem{Table: roomsTable, Item: room},
			common.TableItem{Table: connectionsTable, Item: conn},
		)
		if err != nil {
			return err
		}
	} else {
		resultType = "reject"
	}

	result := map[string]interface{}{
		"type":          resultType,
		"isExistClient": isExistClient,
		"isExistUser":   isExistClient,
	}
	bytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	return api.PostToConnection(connectionID, string(bytes))
}

func handler(request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	ctx := request.RequestContext
	fmt.Println(ctx.ConnectionID, request.Body)

	api, err := common.NewApiGatewayManagementApi(ctx.APIID, ctx.Stage)
	if err != nil {
		return common.ErrorResponse(err, 500)
	}

	err = registerHandler(api, db, ctx.ConnectionID, request.Body, time.Now().UTC())
	if err != nil {
		return common.ErrorResponse(err, 500)
	}

	return events.APIGatewayProxyResponse{
		Body:       "Data sent.",
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}
