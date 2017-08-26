# Get Breeder Data

This utility generates a CSV file containing contact information for various breeders

## Dependencies

```
go get github.com/PuerkitoBio/goquery
```

## Usage

```
go run main.go > output.csv
```

## Flags

|name|description|default|
|---|---|---|
|limit|Amount of records to fetch|1000|
|workers|Number of threads to use to process jobs|4|
