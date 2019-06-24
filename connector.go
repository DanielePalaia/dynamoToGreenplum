package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
)

type awsSession struct {
	dynamoClient     *dynamodbstreams.DynamoDBStreams
	batch            int
	buffer           []string
	gpssclient       *gpssClient
	size             int
	awsTable         string
	awsStreams       int
	greenplumChannel chan []*dynamodbstreams.Record
	batchTimeout     int
	recordsTimeout   int
	count            int
}

func makeAwsSession(region string, awsTable string, endPoint string, batch int, batchTimeout int, recordTimeout int, gpssclient *gpssClient) *awsSession {
	//sess := session.New()
	config := &aws.Config{
		Region: aws.String(region),
	}
	if endPoint != "" {
		config.WithEndpoint(endPoint)
	}
	sess := session.Must(session.NewSession(config))
	mysession := new(awsSession)

	// Exception: we don't want to batch...
	if batch == 0 {
		mysession.buffer = make([]string, 1)
	}

	mysession.size = batch
	mysession.batchTimeout = batchTimeout
	mysession.recordsTimeout = recordTimeout
	mysession.buffer = make([]string, batch)

	mysession.dynamoClient = dynamodbstreams.New(sess)
	mysession.gpssclient = gpssclient
	mysession.awsTable = awsTable
	mysession.awsStreams = 0
	mysession.greenplumChannel = make(chan []*dynamodbstreams.Record)
	mysession.count = 0

	return mysession
}

func (s *awsSession) checkIfSeqFileExists(shardId string) (string, string) {

	filename := "./SeqNumbers/" + shardId
	if _, err := os.Stat("./SeqNumbers/" + shardId); os.IsNotExist(err) {
		// Create the new file
		os.Create("./SeqNumbers/" + shardId)
		return "", filename
	} else {
		file, _ := os.Open("./SeqNumbers/" + shardId)
		// Start reading from the file with a reader.
		reader := bufio.NewReader(file)
		line, _ := reader.ReadString('\n')

		return line, filename
	}

}

// To retrieve all the stream records from a shard
//
// The following example retrieves all the stream records from a shard.
func (s *awsSession) getStreamRecords(streamArn string, shardId string) {

	// Check if seqNumberFile exists
	lastseqnumber, filename := s.checkIfSeqFileExists(shardId)
	for {
		iterator, _ := s.getShardIt(streamArn, shardId, lastseqnumber)
		fmt.Println("Looping for records: ")
		input := &dynamodbstreams.GetRecordsInput{
			ShardIterator: aws.String(*iterator),
		}

		result, err := s.dynamoClient.GetRecords(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case dynamodbstreams.ErrCodeResourceNotFoundException:
					fmt.Println(dynamodbstreams.ErrCodeResourceNotFoundException, aerr.Error())

				case dynamodbstreams.ErrCodeLimitExceededException:
					fmt.Println(dynamodbstreams.ErrCodeLimitExceededException, aerr.Error())

				case dynamodbstreams.ErrCodeInternalServerError:
					fmt.Println(dynamodbstreams.ErrCodeInternalServerError, aerr.Error())

				case dynamodbstreams.ErrCodeExpiredIteratorException:
					fmt.Println(dynamodbstreams.ErrCodeExpiredIteratorException, aerr.Error())

				case dynamodbstreams.ErrCodeTrimmedDataAccessException:
					fmt.Println(dynamodbstreams.ErrCodeTrimmedDataAccessException, aerr.Error())

				default:
					fmt.Println(aerr.Error())

				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())
			}
		}

		if len(result.Records) > 0 {
			s.greenplumChannel <- result.Records
			lastseqnumber = *(result.Records[len(result.Records)-1].Dynamodb.SequenceNumber)
			ioutil.WriteFile(filename, []byte(lastseqnumber), 0644)
		}

		time.Sleep(time.Millisecond * time.Duration(s.recordsTimeout))

	}

}

// To obtain a shard iterator for the provided stream ARN and shard ID
//
// The following example returns a shard iterator for the provided stream ARN and shard
// ID.
func (s *awsSession) getShardIt(streamArn string, label string, seqnumber string) (*string, error) {

	var input *dynamodbstreams.GetShardIteratorInput

	if seqnumber == "" {
		input = &dynamodbstreams.GetShardIteratorInput{
			ShardId:           aws.String(label),
			ShardIteratorType: aws.String("TRIM_HORIZON"),
			StreamArn:         aws.String(streamArn),
		}
	} else {
		input = &dynamodbstreams.GetShardIteratorInput{
			SequenceNumber:    aws.String(seqnumber),
			ShardId:           aws.String(label),
			ShardIteratorType: aws.String("AFTER_SEQUENCE_NUMBER"),
			StreamArn:         aws.String(streamArn),
		}
	}

	result, err := s.dynamoClient.GetShardIterator(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodbstreams.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodbstreams.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodbstreams.ErrCodeInternalServerError:
				fmt.Println(dynamodbstreams.ErrCodeInternalServerError, aerr.Error())
			case dynamodbstreams.ErrCodeTrimmedDataAccessException:
				fmt.Println(dynamodbstreams.ErrCodeTrimmedDataAccessException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return nil, err
	}

	fmt.Println(result)
	return result.ShardIterator, nil
}

func (s *awsSession) getShards(arn string) ([]*dynamodbstreams.Shard, error) {

	input := &dynamodbstreams.DescribeStreamInput{
		StreamArn: aws.String(arn),
	}

	result, err := s.dynamoClient.DescribeStream(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodbstreams.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodbstreams.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodbstreams.ErrCodeInternalServerError:
				fmt.Println(dynamodbstreams.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
			os.Exit(-1)

		}
		return nil, err
	}

	return result.StreamDescription.Shards, err
}

// The following example lists all of the stream ARNs.
func (s *awsSession) getStreamsForTable() (*dynamodbstreams.ListStreamsOutput, error) {

	input := &dynamodbstreams.ListStreamsInput{
		TableName: &s.awsTable,
	}
	result, err := s.dynamoClient.ListStreams(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodbstreams.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodbstreams.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodbstreams.ErrCodeInternalServerError:
				fmt.Println(dynamodbstreams.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return nil, err
	}

	return result, err
}

func (s *awsSession) processStream(stream *dynamodbstreams.Stream) {

	nshards := 0
	mapshards := make(map[string]bool)
	fmt.Println("Processing stream: ")
	//fmt.Println(stream)

	for {
		fmt.Println("looping for new shards to arrive for stream")
		//Get the shards from the stream
		shards, _ := s.getShards(*(stream.StreamArn))
		// New shard arrived
		if len(shards) > nshards {
			fmt.Println("new shards arrived")
			//shardsToProcess := shards[nshards:len(shards)]

			for _, currentshard := range shards {
				if mapshards[*(currentshard.ShardId)] == true {
					continue
				}
				mapshards[*(currentshard.ShardId)] = true
				//fmt.Println(currentshard)
				//iterator, _ := s.getShardIt(*(stream.StreamArn), *(currentshard.ShardId))
				go s.getStreamRecords(*(stream.StreamArn), *(currentshard.ShardId))

			}
			nshards = len(shards)

		}

		time.Sleep(time.Second * 5)

	}

}

func (s *awsSession) ProcessStreams() {

	// Activate the gpss goroutine which will push items on Greenplum through gpss
	ticker := time.NewTicker(time.Duration(s.batchTimeout) * time.Second)
	go s.pushToGreenplum(ticker)

	nstreams := 0
	// stream already processed
	mapstreams := make(map[string]bool)

	// Every 5 sec check if new streams arrive
	for {
		fmt.Println("looping for new streams to arrive")
		streams, _ := s.getStreamsForTable()
		fmt.Printf("number of streams present %d", len(streams.Streams))
		// New streams arrived
		if len(streams.Streams) > nstreams {
			fmt.Println("new streams arrived")

			for _, currentstream := range streams.Streams {
				if mapstreams[*currentstream.StreamArn] == true {
					continue
				}
				mapstreams[*currentstream.StreamArn] = true
				fmt.Printf("looping currentstream: %s  %d\n", currentstream, len(streams.Streams))
				go s.processStream(currentstream)

			}
			nstreams = len(streams.Streams)

		}

		time.Sleep(time.Second * 5)

	}

}

/* This go-routine will just take care of pushing to gpss */
func (s *awsSession) pushToGreenplum(ticker *time.Ticker) {

	fmt.Println("Greenplum goroutine activated")

	for {
		// wait for new records
		select {
		case records := <-s.greenplumChannel:

			for _, rec := range records {

				fmt.Println(rec)

				b, err := json.Marshal(rec)
				if err != nil {
					fmt.Printf("Error: %s", err)
					return
				}

				s.buffer[s.count] = string(b)
				s.count++
				if s.count >= s.size {
					log.Printf("im writing")
					s.gpssclient.ConnectToGreenplumDatabase()
					s.gpssclient.WriteToGreenplum(s.buffer)
					s.gpssclient.DisconnectToGreenplumDatabase()
					s.count = 0
				}

			}
		// Empty the buffer
		case <-ticker.C:
			if s.count < 1 {
				continue
			}
			tmpbuffer := s.buffer[:s.count]
			s.gpssclient.ConnectToGreenplumDatabase()
			s.gpssclient.WriteToGreenplum(tmpbuffer)
			s.gpssclient.DisconnectToGreenplumDatabase()
			s.count = 0

		}
	}

}
