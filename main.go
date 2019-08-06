package main

import (
	"fmt"
	"strconv"
)

func main() {

	prop, _ := ReadPropertiesFile("./properties.ini")
	port, _ := strconv.Atoi(prop["GreenplumPort"])
	logs := prop["logs"]
	gpssClient := MakeGpssClient(prop["GpssAddress"], prop["GreenplumAddress"], int32(port), prop["GreenplumUser"], prop["GreenplumPasswd"], prop["Database"], prop["SchemaName"], prop["TableName"], logs)
	gpssClient.ConnectToGrpcServer()

	regionName := prop["regionName"]
	batch, _ := strconv.Atoi(prop["batch"])
	batchTimeout, _ := strconv.Atoi(prop["batchTimeout"])
	recordTimeout, _ := strconv.Atoi(prop["recordTimeout"])
	endPoint := prop["endPoint"]
	awsTable := prop["AwsTableName"]
	awssession := makeAwsSession(regionName, awsTable, endPoint, batch, batchTimeout, recordTimeout, gpssClient, logs)

	fmt.Println("Connector started all logs: " + logs)
	awssession.ProcessStreams()
}
