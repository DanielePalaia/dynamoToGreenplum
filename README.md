# Introduction
This software is a sample project that shows how it is possible to create a GPSS client for Greenplum Streaming Server. </br>
We will go through all the necessary phases needed to build it </br>
This application is using the following technologies: DynamoDB streams, GRPC, GO and Greenplum Streaming Server (GPSS) </br>
The following reading can be useful to understand the scenario: </br></br>
**DynamoDB streams:** </br>
https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html </br>
**GRPC:**  </br>
https://grpc.io/ </br>
**Greenplum GPSS:**</br>
https://gpdb.docs.pivotal.io/5160/greenplum-stream/api/dev_client.html</br>

## Summary
The scenario we are building is the following: </br>We are receiving some dynamodb streams, initially all the ones already created and then
we will wait for new ones to be generated and then we will do ingestions on a Greenplum Database table through GPSS.</br>
DynamoDB stream records will be stored as .json in a Greenplum table to allow maximum flexibility on them </br>

### How to create a gpss app:
GPSS is based on GRPC, a remote procedure call mechanism where from a contract (.proto file) you can generate automatically code that the client can use. The .proto of GPSS can be found in: </br>
</br>https://gpdb.docs.pivotal.io/5160/greenplum-stream/api/svcdef.html</br></br>
You can use whatever programming language supported by GRPC: Java, C++, Go ecc.. </br>
Code in GO was already generated by me and you can find it in ./proto directory and reuse it. You will see that the interface supported by the server is this one: </br>
 </br>
type GpssServer interface { </br>
	// Establish a connection to Greenplum Database; returns a Session object </br>
	Connect(context.Context, *ConnectRequest) (*Session, error) </br>
	// Disconnect, freeing all resources allocated for a session </br>
	Disconnect(context.Context, *Session) (*empty.Empty, error) </br>
	// Prepare and open a table for write </br>
	Open(context.Context, *OpenRequest) (*empty.Empty, error) </br>
	// Write data to table </br>
	Write(context.Context, *WriteRequest) (*empty.Empty, error) </br>
	// Close a write operation </br>
	Close(context.Context, *CloseRequest) (*TransferStats, error) </br>
	// List all available schemas in a database </br>
	ListSchema(context.Context, *ListSchemaRequest) (*Schemas, error) </br>
	// List all tables and views in a schema </br>
	ListTable(context.Context, *ListTableRequest) (*Tables, error) </br>
	// Decribe table metadata(column name and column type) </br>
	DescribeTable(context.Context, *DescribeTableRequest) (*Columns, error) </br>
}  </br></br>
So these are the request you can send to the gpss server at the moment. Just include this package on your app and you can use them </br>
On top of this I created a library to compose requests and call this interface which can be found in gpssfunc.go and can be resued or takes as template </br></br>

## Design about the software

The software is using the dynamodb streams api to collect the stream recors from dynamo db. </br>
https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_Operations_Amazon_DynamoDB_Streams.html </br>
The software will listen for new streams, shards o records created (will consider also the one already present) </br>
When a new stream appears it creates a new goroutine for it to manage new shards and records </br>
Also a dedicated goroutine will take care to send batch of informations to GPSS </br>
Items are sent to GPSS in batches configurable at input. When a certain amount of records is received from dynamodb a request will be activated to GPSS for ingestion</br>

## Running the app:
### Prerequisites
1. **Install DynamoDB and aws-cli (if you don't have aws access)** </br></br>
   I tried the software locally, so first phase is to install dynamodb and aws-cli, creating a table supporting streams and generate some streams
   (for example doing some inserts). Like this:</br></br>
   **/Users/dpalaia/Library/Python/3.7/bin/aws dynamodb create-table     --table-name MusicCollection     --attribute-definitions         AttributeName=Artist,AttributeType=S AttributeName=SongTitle,AttributeType=S     --key-schema AttributeName=Artist,KeyType=HASH AttributeName=SongTitle,KeyType=RANGE     --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1  --stream-specification StreamEnabled=true,StreamViewType=NEW_AND_OLD_IMAGES --endpoint-url http://localhost:8000 --region x**
   </br></br>
   Then generate some streams:</br></br>
   **/Users/dpalaia/Library/Python/3.7/bin/aws dynamodb put-item     --table-name MusicCollection     --item '{"Artist": {"S":"item_8"},"SongTitle": {"S":"Song Valuexcv 8"}}'     --region x --endpoint-url http://localhost:8000**   
  
2. **Running GPSS** </br></br>
Run a GPSS server instance on Greenplum side: </br>
**gpss gpss.conf**</br>
where gpss.conf is </br></br>
{</br>
    "ListenAddress": {</br>
        "Host": "",</br>
        "Port": 50007,</br>
        "SSL": false</br>
    },</br>
    "Gpfdist": {</br>
        "Host": "",</br>
        "Port": 8113</br>
    }</br>
}</br>

3. **Creating a Greenplum table** </br></br>
Create a Greenplum table with just a json data field to contain elements from DynamoDB streams </br>
**CREATE TABLE test(data json);** </br>

### Running:
The app is written in GO. Binaries files for osx and linux are already provided inside the folder ./bin </br>

1. There is an initialization file properties.ini which needs to be filled before running the software: </br>

    **GpssAddress=172.16.125.152:50007**</br> 
    **GreenplumAddress=172.16.125.152**</br>
    **GreenplumPort=5432**</br>
    **GreenplumUser=gpadmin**</br>
    **GreenplumPasswd=**</br>
    **Database=dashboard**</br>
    **SchemaName=public**</br>
    **AwsTableName=MusicCollection**</br>
    **TableName=dynamosimulation2**</br>
    **batch=4**</br>
    **regionName=M**</br>
    **endPoint=http://localhost:8000**</br>
    
    endPoint may be used if running locally (in this case put the same region you used during dynamodb insert), otherwise specify just a valid aws region
  
 2. After it simply run the binary</br>
 ./dynamoToGreenplum </br>
 
 Once runned the software will begin to search for existing streams and ingest records (if any) </br>
 After, it will wait for new streams or records to be generated (every 5sec) </br>

## Compiling
you may want to compile the app. In this case you need a go compiler.</br>
Put the source in your $GOPATH/src directory</br>
You need to download aws libraries, do as following:</br></br>
go get github.com/aws/aws-sdk-go/aws</br>
go get github.com/aws/aws-sdk-go/aws/awserr</br>
go get github.com/aws/aws-sdk-go/aws/session</br>
go get github.com/aws/aws-sdk-go/service/dynamodbstreams</br></br>
The just run a go build or go install to produce the binary
