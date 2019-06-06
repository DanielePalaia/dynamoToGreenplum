package main

import (
	"strconv"
)

func main() {

	prop, _ := ReadPropertiesFile("./properties.ini")
	port, _ := strconv.Atoi(prop["GreenplumPort"])
	gpssClient := MakeGpssClient(prop["GpssAddress"], prop["GreenplumAddress"], int32(port), prop["GreenplumUser"], prop["GreenplumPasswd"], prop["Database"], prop["SchemaName"], prop["TableName"])
	gpssClient.ConnectToGrpcServer()

	regionName := prop["regionName"]
	batch, _ := strconv.Atoi(prop["batch"])
	endPoint := prop["endPoint"]
	awsTable := prop["AwsTableName"]
	awssession := makeAwsSession(regionName, awsTable, endPoint, batch, gpssClient)

	awssession.ProcessStreams()
}
