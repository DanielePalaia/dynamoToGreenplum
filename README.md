# Introduction
This software is a template project that shows how it is possible to create a GPSS client for Greenplum Streaming Server. </br>
We will go through all the necessary phases needed to build it </br>
This application is using the following technologies: DynamoDB streams, GRPC, GO and Greenplum Streaming Server (GPSS) </br>
The following reading can be useful to understand the scenario: </br></br>
**DynamoDB streams:** </br>
https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html </br>
**GRPC:**  </br>
https://grpc.io/ </br>
**Greenplum GPSS:**</br>
https://gpdb.docs.pivotal.io/5160/greenplum-stream/overview.html</br>
https://gpdb.docs.pivotal.io/5160/greenplum-stream/api/dev_client.html</br>

## Description of the project
The scenario we are building is the following: </br>We are receiving some dynamodb streams, initially all the ones already created and then
we will wait for new ones to be generated and then we will do ingestions on a Greenplum Database table through GPSS.</br>
DynamoDB stream records will be stored as .json in a Greenplum table to allow maximum flexibility on them </br>
**Update: an update was made since this initial version. Now in a file in ./bin/linux/SeqNumber directory a record sequence number will be stored so that if the software is interrupted or restarted it will start ingesting from the last item previously ingested rather than start again from beginning**

## How to create a gpss app
GPSS is based on GRPC, a remote procedure call mechanism where from a contract (.proto file) you can generate automatically code that the client can use. The .proto of GPSS can be found in: </br>
</br>https://gpdb.docs.pivotal.io/5160/greenplum-stream/api/svcdef.html</br></br>
Code can be automatically generated with the protoc tool ex:</br>
https://grpc.io/docs/quickstart/go/</br>
You can use whatever programming language supported by GRPC: Java, C++, Go ecc.. </br>
Code in GO was already generated by me and you can find it in ./proto directory and reuse it. You will see that the interface supported by the server is this one: </br>
 </br>
 
```
type GpssServer interface { </br>
	// Establish a connection to Greenplum Database; returns a Session object </br>
	__Connect(context.Context, *ConnectRequest) (*Session, error)__ </br>
	// Disconnect, freeing all resources allocated for a session </br>
	__Disconnect(context.Context, *Session) (*empty.Empty, error)__ </br>
	// Prepare and open a table for write </br>
	__Open(context.Context, *OpenRequest) (*empty.Empty, error)__ </br>
	// Write data to table </br>
	__Write(context.Context, *WriteRequest) (*empty.Empty, error)__ </br>
	// Close a write operation </br>
	__Close(context.Context, *CloseRequest) (*TransferStats, error)__ </br>
	// List all available schemas in a database </br>
	__ListSchema(context.Context, *ListSchemaRequest) (*Schemas, error)__ </br>
	// List all tables and views in a schema </br>
	__ListTable(context.Context, *ListTableRequest) (*Tables, error)__ </br>
	// Decribe table metadata(column name and column type) </br>
	__DescribeTable(context.Context, *DescribeTableRequest) (*Columns, error)__ </br>
}  </br></br>
```

So these are the request you can send to the gpss server at the moment. Just include this package on your app and you can use them </br>
On top of this I created a library to compose requests and call this interface which can be found in gpssfunc.go and can be resued or taken as template for future work</br></br>

## Design of the software

The software is using the dynamodb streams api to collect the stream recors from dynamo db. </br>
https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_Operations_Amazon_DynamoDB_Streams.html </br>
The software will listen for new streams, shards o records created (will consider also the one already present) </br>
When a new stream appears it creates a new goroutine for it to manage new shards and records </br>
Also a dedicated goroutine will take care to send batch of informations to GPSS </br>
Items are sent to GPSS in batches configurable at input. When a certain amount of records is received from dynamodb a request will be activated to GPSS for ingestion</br>

## Running the app
### Prerequisites
1. **Install DynamoDB and aws-cli (if you don't have aws access)** </br></br>
   I tried the software locally, so first phase is to install dynamodb and aws-cli, creating a table supporting streams and generate some streams
   (for example doing some inserts). Like this:</br></br>
   /Users/dpalaia/Library/Python/3.7/bin/aws dynamodb create-table     --table-name MusicCollection     --attribute-definitions         AttributeName=Artist,AttributeType=S AttributeName=SongTitle,AttributeType=S     --key-schema AttributeName=Artist,KeyType=HASH AttributeName=SongTitle,KeyType=RANGE     --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1  --stream-specification StreamEnabled=true,StreamViewType=NEW_AND_OLD_IMAGES --endpoint-url http://localhost:8000 --region x
   </br></br>
   Then generate some streams:</br></br>
   /Users/dpalaia/Library/Python/3.7/bin/aws dynamodb put-item     --table-name MusicCollection     --item '{"Artist": {"S":"item_8"},"SongTitle": {"S":"Song Valuexcv 8"}}'     --region x --endpoint-url http://localhost:8000   
   </br>
   Also, please configure your $HOME/.aws/credentials to put your aws credential (if you use it locally you can just put random values ) like </br></br>
   [default]</br>
aws_access_key_id=AKIAIOSFODNN7EXAMPLE</br>
aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
  
2. **Running GPSS** </br></br>
Run a GPSS server instance on Greenplum side: </br>
**gpss gpss.conf**</br>
where gpss.conf is </br></br>
```
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
```

3. **Creating a Greenplum table** </br></br>
Create a Greenplum table with just a json data field to contain elements from DynamoDB streams </br>
```
CREATE TABLE dynamosimulation2(data json); </br>
```

### Running
The app is written in GO. Binaries files for osx and linux are already provided inside the folder ./bin </br>

1. There is an initialization file properties.ini which needs to be filled before running the software: </br>

    ```
    GpssAddress=172.16.125.152:50007**</br> 
    GreenplumAddress=172.16.125.152**</br>
    GreenplumPort=5432**</br>
    GreenplumUser=gpadmin**</br>
    GreenplumPasswd=**</br>
    Database=dashboard**</br>
    SchemaName=public**</br>
    AwsTableName=MusicCollection**</br>
    TableName=dynamosimulation2**</br>
    batch=4**</br>
    batchTimeout=5**</br>
    recordTimeout=1000**  
    regionName=M**</br>
    endPoint=http://localhost:8000**</br>
    ```
    
endPoint may be used if running locally (in this case put the same region you used during dynamodb insert), otherwise specify just a valid aws region.
Batch will buff items before sending the request to Greenplum (if set to 1 is immediate)</br>
**Update2: batchtimeout expressed in seconds will store the batch elements even if we didn't reach the batch values (to avoid elements to be stored forever in memory if we don't receive others**</br>
**batchTimeout expressed in milliseconds is the pause we set every GetRecords is executed to check for updateds, lower values will consume more cpu**</br>
      
 2. After it simply run the binary</br>
 ./dynamoToGreenplum </br>
 
 Once runned the software will begin to search for existing streams and ingest records (if any) </br>
 After, it will wait for new streams or records to be generated (every 5sec) </br>
 So you can add new insert in the table to generate new records to be processed like before try:</br></br>
/Users/dpalaia/Library/Python/3.7/bin/aws dynamodb put-item     --table-name MusicCollection     --item '{"Artist": {"S":"item_8"},"SongTitle": {"S":"Song Valuexcv 8"}}'     --region x --endpoint-url http://localhost:8000 

## Compiling
you may want to compile the app. In this case you need a go compiler.</br>
Put the source in your $GOPATH/src directory</br>
You need to download aws libraries, do as following:</br></br>
go get github.com/aws/aws-sdk-go/aws</br>
go get github.com/aws/aws-sdk-go/aws/awserr</br>
go get github.com/aws/aws-sdk-go/aws/session</br>
go get github.com/aws/aws-sdk-go/service/dynamodbstreams</br></br>
Also the gpss package which contains the protobuf specification (the autogenerated one) should be also put in your $GOPATH/src/gpssclient </br>
Then just run a go build or go install to produce the binary
