package goh

import (
	"fmt"
	"github.com/sdming/goh/Hbase"
	"net"
	"net/url"
	"thrift"
)

type HClient struct {
	//Scheme string
	//Host            string
	//Port            int
	addr            string
	Protocol        int
	Trans           thrift.TTransport
	ProtocolFactory thrift.TProtocolFactory
	client          *Hbase.HbaseClient
	state           int //
}

/*
NewHttpClient return a hbase http client instance

*/
func NewHttpClient(rawurl string, protocol int) (client *HClient, err error) {
	//var client *HClient

	parsedUrl, err := url.Parse(rawurl)
	if err != nil {
		//return client, err
		return
	}

	trans, err := thrift.NewTHttpClient(parsedUrl.String())
	if err != nil {
		//return client, err
		return
	}

	return NewClient(parsedUrl.String(), protocol, trans)
}

/*
NewTcpClient return a base tcp client instance

*/
func NewTcpClient(addr string, port string, protocol int, framed bool) (client *HClient, err error) {
	//var client *HClient

	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprint(addr, ":", port))
	if err != nil {
		//return client, err
		return
	}

	var trans thrift.TTransport
	trans, err = thrift.NewTNonblockingSocketAddr(tcpAddr)
	if err != nil {
		//return client, err
		return
	}
	if framed {
		trans = thrift.NewTFramedTransport(trans)
	}

	return NewClient(tcpAddr.String(), protocol, trans)
}

func NewClient(addr string, protocol int, trans thrift.TTransport) (*HClient, error) {
	var client *HClient

	protocolFactory, err := newProtocolFactory(protocol)
	if err != nil {
		return client, err
	}

	client = &HClient{
		addr:            addr,
		Protocol:        protocol,
		ProtocolFactory: protocolFactory,
		Trans:           trans,
		client:          Hbase.NewHbaseClientFactory(trans, protocolFactory),
	}

	if err = client.Open(); err != nil {
		return nil, err
	}

	return client, nil

}

func (client *HClient) Open() error {
	if client.state == stateDefault {
		if err := client.Trans.Open(); err != nil {
			return err
		}
		fmt.Println("open")
		client.state = stateOpen
	}
	return nil
}

func (client *HClient) Close() error {
	if client.state == stateOpen {
		if err := client.Trans.Close(); err != nil {
			return err
		}
		fmt.Println("close")
		client.state = stateDefault
	}
	return nil
}

/**
 * Brings a table on-line (enables it)
 * 
 * Parameters:
 *  - TableName: name of the table
 */
func (client *HClient) EnableTable(tableName string) error {
	if err := client.Open(); err != nil {
		return err
	}

	return checkHbaseError(client.client.EnableTable(Hbase.Bytes(tableName)))
}

/**
 * Disables a table (takes it off-line) If it is being served, the master
 * will tell the servers to stop serving it.
 * 
 * Parameters:
 *  - TableName: name of the table
 */
func (client *HClient) DisableTable(tableName string) (err error) {
	if err = client.Open(); err != nil {
		return
	}

	return checkHbaseError(client.client.DisableTable(Hbase.Bytes(tableName)))
}

/**
 * @return true if table is on-line
 * 
 * Parameters:
 *  - TableName: name of the table to check
 */
func (client *HClient) IsTableEnabled(tableName string) (ret bool, err error) {
	if err = client.Open(); err != nil {
		return
	}

	ret, io, e1 := client.client.IsTableEnabled(Hbase.Bytes(tableName))
	err = checkHbaseError(io, e1)
	return
}

/**
 * Parameters:
 *  - TableNameOrRegionName
 */
func (client *HClient) Compact(tableNameOrRegionName string) (err error) {
	if err = client.Open(); err != nil {
		return
	}

	return checkHbaseError(client.client.Compact(Hbase.Bytes(tableNameOrRegionName)))
}

/**
 * Parameters:
 *  - TableNameOrRegionName
 */
func (client *HClient) MajorCompact(tableNameOrRegionName string) (err error) {
	if err = client.Open(); err != nil {
		return
	}

	return checkHbaseError(client.client.MajorCompact(Hbase.Bytes(tableNameOrRegionName)))
}

/**
 * List all the column families assoicated with a table.
 * 
 * @return list of column family descriptors
 * 
 * Parameters:
 *  - TableName: table name
 */
func (client *HClient) GetTableNames() (tables []string, err error) {
	if err = client.Open(); err != nil {
		return
	}

	ret, io, e1 := client.client.GetTableNames()
	if err = checkHbaseError(io, e1); err != nil {
		return
	}

	l := ret.Len()
	tables = make([]string, l)
	for i := 0; i < l; i++ {
		tables[i] = ret.At(i).(string)
	}

	return
}

/**
 * List all the column families assoicated with a table.
 * 
 * @return list of column family descriptors
 * 
 * Parameters:
 *  - TableName: table name
 */
func (client *HClient) GetColumnDescriptors(tableName string) (columns map[string]*ColumnDescriptor, err error) {
	if err = client.Open(); err != nil {
		return
	}

	ret, io, e1 := client.client.GetColumnDescriptors(Hbase.Text(tableName))
	if err = checkHbaseError(io, e1); err != nil {
		return
	}

	l := ret.Len()
	columns = make(map[string]*ColumnDescriptor, l)
	//fmt.Println("KeyType", ret.KeyType())
	//fmt.Println("ValueType", ret.ValueType())
	//fmt.Println("len", l)

	keys := ret.Keys()
	for i := 0; i < l; i++ {
		key := keys[i]
		value, ok := ret.Get(key)

		if !ok {
			continue
		}

		hbaseColumn := value.(*Hbase.ColumnDescriptor)
		column := &ColumnDescriptor{
			Name:                  string(hbaseColumn.Name),
			MaxVersions:           hbaseColumn.MaxVersions,
			Compression:           hbaseColumn.Compression,
			InMemory:              hbaseColumn.InMemory,
			BloomFilterType:       hbaseColumn.BloomFilterType,
			BloomFilterVectorSize: hbaseColumn.BloomFilterVectorSize,
			BloomFilterNbHashes:   hbaseColumn.BloomFilterNbHashes,
			BlockCacheEnabled:     hbaseColumn.BlockCacheEnabled,
			TimeToLive:            hbaseColumn.TimeToLive,
		}

		columns[column.Name] = column
	}

	return
}

/**
 * List the regions associated with a table.
 * 
 * @return list of region descriptors
 * 
 * Parameters:
 *  - TableName: table name
 */
func (client *HClient) GetTableRegions(tableName string) (regions []*TRegionInfo, err error) {
	if err = client.Open(); err != nil {
		return
	}

	ret, io, e1 := client.client.GetTableRegions(Hbase.Text(tableName))
	if err = checkHbaseError(io, e1); err != nil {
		return
	}

	l := ret.Len()
	regions = make([]*TRegionInfo, l)
	//fmt.Println("ElemType", ret.ElemType())
	//fmt.Println("len", l)

	for i := 0; i < l; i++ {
		value := ret.At(i)
		//fmt.Println(value)
		hbaseRegion := value.(*Hbase.TRegionInfo)

		region := &TRegionInfo{
			StartKey:   string(hbaseRegion.StartKey),
			EndKey:     string(hbaseRegion.EndKey),
			Id:         hbaseRegion.Id,
			Name:       string(hbaseRegion.Name),
			Version:    hbaseRegion.Version,
			ServerName: string(hbaseRegion.ServerName),
			Port:       hbaseRegion.Port,
		}
		regions[i] = region
	}

	return
}

/**
 * Create a table with the specified column families.  The name
 * field for each ColumnDescriptor must be set and must end in a
 * colon (:). All other fields are optional and will get default
 * values if not explicitly specified.
 * 
 * @throws IllegalArgument if an input parameter is invalid
 * 
 * @throws AlreadyExists if the table name already exists
 * 
 * Parameters:
 *  - TableName: name of table to create
 *  - ColumnFamilies: list of column family descriptors
 */
func (client *HClient) CreateTable(tableName string, columnFamilies []*ColumnDescriptor) (err error) {
	if err = client.Open(); err != nil {
		return
	}

	l := len(columnFamilies)
	columns := thrift.NewTList(thrift.STRUCT, l)
	for i := 0; i < l; i++ {
		col := columnFamilies[i]
		hbaseColumn := &Hbase.ColumnDescriptor{
			Name:                  Hbase.Text(col.Name),
			MaxVersions:           col.MaxVersions,
			Compression:           col.Compression,
			InMemory:              col.InMemory,
			BloomFilterType:       col.BloomFilterType,
			BloomFilterVectorSize: col.BloomFilterVectorSize,
			BloomFilterNbHashes:   col.BloomFilterNbHashes,
			BlockCacheEnabled:     col.BlockCacheEnabled,
			TimeToLive:            col.TimeToLive,
		}
		columns.Set(i, hbaseColumn)
	}

	io, ia, exist, e1 := client.client.CreateTable(Hbase.Text(tableName), columns)
	err = checkHbaseError(io, e1)

	return
}

/**
 * Deletes a table
 * 
 * @throws IOError if table doesn't exist on server or there was some other
 * problem
 * 
 * Parameters:
 *  - TableName: name of table to delete
 */
func (client *HClient) DeleteTable(tableName string) error {
	fmt.Println(tableName)
	return nil
}

/**
 * Get a single TCell for the specified table, row, and column at the
 * latest timestamp. Returns an empty list if no such value exists.
 * 
 * @return value for specified row/column
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Column: column name
 *  - Attributes: Get attributes
 */
func (client *HClient) Get(tableName string, row string, column string, attributes map[string]string) ([]TCell, error) {
	fmt.Println(tableName, row, column, attributes)
	return nil, nil
}

/**
 * Get the specified number of versions for the specified table,
 * row, and column.
 * 
 * @return list of cells for specified row/column
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Column: column name
 *  - NumVersions: number of versions to retrieve
 *  - Attributes: Get attributes
 */
func (client *HClient) GetVer(tableName string, row string, column string, numVersions int32, attributes map[string]string) ([]TCell, error) {
	fmt.Println(tableName, row, column, numVersions, attributes)
	return nil, nil
}

/**
 * Get the specified number of versions for the specified table,
 * row, and column.  Only versions less than or equal to the specified
 * timestamp will be returned.
 * 
 * @return list of cells for specified row/column
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Column: column name
 *  - Timestamp: timestamp
 *  - NumVersions: number of versions to retrieve
 *  - Attributes: Get attributes
 */
func (client *HClient) GetVerTs(tableName string, row string, column string, timestamp int64, numVersions int32, attributes map[string]string) ([]TCell, error) {
	fmt.Println(tableName, row, column, timestamp, numVersions, attributes)
	return nil, nil
}

/**
 * Get all the data for the specified table and row at the latest
 * timestamp. Returns an empty list if the row does not exist.
 * 
 * @return TRowResult containing the row and map of columns to TCells
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Attributes: Get attributes
 */
func (client *HClient) GetRow(tableName string, row string, attributes map[string]string) ([]TRowResult, error) {
	fmt.Println(tableName, row, attributes)
	return nil, nil
}

/**
 * Get the specified columns for the specified table and row at the latest
 * timestamp. Returns an empty list if the row does not exist.
 * 
 * @return TRowResult containing the row and map of columns to TCells
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Columns: List of columns to return, null for all columns
 *  - Attributes: Get attributes
 */
func (client *HClient) GetRowWithColumns(tableName string, row string, columns []string, attributes map[string]string) ([]TRowResult, error) {
	fmt.Println(tableName, row, columns, attributes)
	return nil, nil
}

/**
 * Get all the data for the specified table and row at the specified
 * timestamp. Returns an empty list if the row does not exist.
 * 
 * @return TRowResult containing the row and map of columns to TCells
 * 
 * Parameters:
 *  - TableName: name of the table
 *  - Row: row key
 *  - Timestamp: timestamp
 *  - Attributes: Get attributes
 */
func (client *HClient) GetRowTs(tableName string, row string, timestamp int64, attributes map[string]string) ([]TRowResult, error) {
	fmt.Println(tableName, row, timestamp, attributes)
	return nil, nil
}

/**
 * Get the specified columns for the specified table and row at the specified
 * timestamp. Returns an empty list if the row does not exist.
 * 
 * @return TRowResult containing the row and map of columns to TCells
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Columns: List of columns to return, null for all columns
 *  - Timestamp
 *  - Attributes: Get attributes
 */
func (client *HClient) GetRowWithColumnsTs(tableName string, row string, columns []string, timestamp int64, attributes map[string]string) ([]TRowResult, error) {
	fmt.Println(tableName, row, columns, timestamp, attributes)
	return nil, nil
}

/**
 * Get all the data for the specified table and rows at the latest
 * timestamp. Returns an empty list if no rows exist.
 * 
 * @return TRowResult containing the rows and map of columns to TCells
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Rows: row keys
 *  - Attributes: Get attributes
 */
func (client *HClient) GetRows(tableName string, rows []string, attributes map[string]string) ([]TRowResult, error) {
	fmt.Println(tableName, rows, attributes)
	return nil, nil
}

/**
 * Get the specified columns for the specified table and rows at the latest
 * timestamp. Returns an empty list if no rows exist.
 * 
 * @return TRowResult containing the rows and map of columns to TCells
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Rows: row keys
 *  - Columns: List of columns to return, null for all columns
 *  - Attributes: Get attributes
 */
func (client *HClient) GetRowsWithColumns(tableName string, rows []string, columns []string, attributes map[string]string) ([]TRowResult, error) {
	fmt.Println(tableName, rows, columns, attributes)
	return nil, nil
}

/**
 * Get all the data for the specified table and rows at the specified
 * timestamp. Returns an empty list if no rows exist.
 * 
 * @return TRowResult containing the rows and map of columns to TCells
 * 
 * Parameters:
 *  - TableName: name of the table
 *  - Rows: row keys
 *  - Timestamp: timestamp
 *  - Attributes: Get attributes
 */
func (client *HClient) GetRowsTs(tableName string, rows []string, timestamp int64, attributes map[string]string) ([]TRowResult, error) {
	fmt.Println(tableName, rows, timestamp, attributes)
	return nil, nil
}

/**
 * Get the specified columns for the specified table and rows at the specified
 * timestamp. Returns an empty list if no rows exist.
 * 
 * @return TRowResult containing the rows and map of columns to TCells
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Rows: row keys
 *  - Columns: List of columns to return, null for all columns
 *  - Timestamp
 *  - Attributes: Get attributes
 */
func (client *HClient) GetRowsWithColumnsTs(tableName string, rows []string, columns []string, timestamp int64, attributes map[string]string) ([]TRowResult, error) {
	fmt.Println(tableName, rows, columns, timestamp, attributes)
	return nil, nil
}

/**
 * Apply a series of mutations (updates/deletes) to a row in a
 * single transaction.  If an exception is thrown, then the
 * transaction is aborted.  Default current timestamp is used, and
 * all entries will have an identical timestamp.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Mutations: list of mutation commands
 *  - Attributes: Mutation attributes
 */
func (client *HClient) MutateRow(tableName string, row string, mutations []Mutation, attributes map[string]string) error {
	fmt.Println(tableName, row, mutations, attributes)
	return nil
}

/**
 * Apply a series of mutations (updates/deletes) to a row in a
 * single transaction.  If an exception is thrown, then the
 * transaction is aborted.  The specified timestamp is used, and
 * all entries will have an identical timestamp.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Mutations: list of mutation commands
 *  - Timestamp: timestamp
 *  - Attributes: Mutation attributes
 */
func (client *HClient) MutateRowTs(tableName string, row string, mutations []Mutation, timestamp int64, attributes map[string]string) error {
	fmt.Println(tableName, row, mutations, timestamp, attributes)
	return nil
}

/**
 * Apply a series of batches (each a series of mutations on a single row)
 * in a single transaction.  If an exception is thrown, then the
 * transaction is aborted.  Default current timestamp is used, and
 * all entries will have an identical timestamp.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - RowBatches: list of row batches
 *  - Attributes: Mutation attributes
 */
func (client *HClient) MutateRows(tableName string, rowBatches []BatchMutation, attributes map[string]string) error {
	fmt.Println(tableName, rowBatches, attributes)
	return nil
}

/**
 * Apply a series of batches (each a series of mutations on a single row)
 * in a single transaction.  If an exception is thrown, then the
 * transaction is aborted.  The specified timestamp is used, and
 * all entries will have an identical timestamp.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - RowBatches: list of row batches
 *  - Timestamp: timestamp
 *  - Attributes: Mutation attributes
 */
func (client *HClient) MutateRowsTs(tableName string, rowBatches []BatchMutation, timestamp int64, attributes map[string]string) error {
	fmt.Println(tableName, rowBatches, timestamp, attributes)
	return nil
}

/**
 * Atomically increment the column value specified.  Returns the next value post increment.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row to increment
 *  - Column: name of column
 *  - Value: amount to increment by
 */
func (client *HClient) AtomicIncrement(tableName string, row string, column string, value int64) (int64, error) {
	fmt.Println(tableName, row, column, value)
	return 0, nil
}

/**
 * Delete all cells that match the passed row and column.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: Row to update
 *  - Column: name of column whose value is to be deleted
 *  - Attributes: Delete attributes
 */
func (client *HClient) DeleteAll(tableName string, row string, column string, attributes map[string]string) error {
	fmt.Println(tableName, row, column, attributes)
	return nil
}

/**
 * Delete all cells that match the passed row and column and whose
 * timestamp is equal-to or older than the passed timestamp.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: Row to update
 *  - Column: name of column whose value is to be deleted
 *  - Timestamp: timestamp
 *  - Attributes: Delete attributes
 */
func (client *HClient) DeleteAllTs(tableName string, row string, column string, timestamp int64, attributes map[string]string) error {
	fmt.Println(tableName, row, column, timestamp, attributes)
	return nil
}

/**
 * Completely delete the row's cells.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: key of the row to be completely deleted.
 *  - Attributes: Delete attributes
 */
func (client *HClient) DeleteAllRow(tableName string, row string, attributes map[string]string) error {
	fmt.Println(tableName, row, attributes)
	return nil
}

/**
 * Increment a cell by the ammount.
 * Increments can be applied async if hbase.regionserver.thrift.coalesceIncrement is set to true.
 * False is the default.  Turn to true if you need the extra performance and can accept some
 * data loss if a thrift server dies with increments still in the queue.
 * 
 * Parameters:
 *  - Increment: The single increment to apply
 */
func (client *HClient) Increment(increment *TIncrement) error {
	fmt.Println(increment)
	return nil
}

/**
 * Parameters:
 *  - Increments: The list of increments
 */
func (client *HClient) IncrementRows(increments []*TIncrement) error {
	fmt.Println(increments)
	return nil
}

/**
 * Completely delete the row's cells marked with a timestamp
 * equal-to or older than the passed timestamp.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: key of the row to be completely deleted.
 *  - Timestamp: timestamp
 *  - Attributes: Delete attributes
 */
func (client *HClient) DeleteAllRowTs(tableName string, row string, timestamp int64, attributes map[string]string) error {
	fmt.Println(tableName, row, timestamp, attributes)
	return nil
}

/**
 * Get a scanner on the current table, using the Scan instance
 * for the scan parameters.
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Scan: Scan instance
 *  - Attributes: Scan attributes
 */
func (client *HClient) ScannerOpenWithScan(tableName string, scan *TScan, attributes map[string]string) (ScannerID, error) {
	fmt.Println(tableName, scan, attributes)
	return ScannerID(0), nil
}

/**
 * Get a scanner on the current table starting at the specified row and
 * ending at the last row in the table.  Return the specified columns.
 * 
 * @return scanner id to be used with other scanner procedures
 * 
 * Parameters:
 *  - TableName: name of table
 *  - StartRow: Starting row in table to scan.
 * Send "" (empty string) to start at the first row.
 *  - Columns: columns to scan. If column name is a column family, all
 * columns of the specified column family are returned. It's also possible
 * to pass a regex in the column qualifier.
 *  - Attributes: Scan attributes
 */
func (client *HClient) ScannerOpen(tableName string, startRow string, columns []string, attributes map[string]string) (ScannerID, error) {
	fmt.Println(tableName, startRow, columns, attributes)
	return ScannerID(0), nil
}

/**
 * Get a scanner on the current table starting and stopping at the
 * specified rows.  ending at the last row in the table.  Return the
 * specified columns.
 * 
 * @return scanner id to be used with other scanner procedures
 * 
 * Parameters:
 *  - TableName: name of table
 *  - StartRow: Starting row in table to scan.
 * Send "" (empty string) to start at the first row.
 *  - StopRow: row to stop scanning on. This row is *not* included in the
 * scanner's results
 *  - Columns: columns to scan. If column name is a column family, all
 * columns of the specified column family are returned. It's also possible
 * to pass a regex in the column qualifier.
 *  - Attributes: Scan attributes
 */
func (client *HClient) ScannerOpenWithStop(tableName string, startRow string, stopRow string, columns []string, attributes map[string]string) (ScannerID, error) {
	fmt.Println(tableName, startRow, stopRow, columns, attributes)
	return ScannerID(0), nil
}

/**
 * Open a scanner for a given prefix.  That is all rows will have the specified
 * prefix. No other rows will be returned.
 * 
 * @return scanner id to use with other scanner calls
 * 
 * Parameters:
 *  - TableName: name of table
 *  - StartAndPrefix: the prefix (and thus start row) of the keys you want
 *  - Columns: the columns you want returned
 *  - Attributes: Scan attributes
 */
func (client *HClient) ScannerOpenWithPrefix(tableName string, startAndPrefix string, columns []string, attributes map[string]string) (ScannerID, error) {
	fmt.Println(tableName, startAndPrefix, columns, attributes)
	return ScannerID(0), nil
}

/**
 * Get a scanner on the current table starting at the specified row and
 * ending at the last row in the table.  Return the specified columns.
 * Only values with the specified timestamp are returned.
 * 
 * @return scanner id to be used with other scanner procedures
 * 
 * Parameters:
 *  - TableName: name of table
 *  - StartRow: Starting row in table to scan.
 * Send "" (empty string) to start at the first row.
 *  - Columns: columns to scan. If column name is a column family, all
 * columns of the specified column family are returned. It's also possible
 * to pass a regex in the column qualifier.
 *  - Timestamp: timestamp
 *  - Attributes: Scan attributes
 */
func (client *HClient) ScannerOpenTs(tableName string, startRow string, columns []string, timestamp int64, attributes map[string]string) (ScannerID, error) {
	fmt.Println(tableName, startRow, columns, timestamp, attributes)
	return ScannerID(0), nil
}

/**
 * Get a scanner on the current table starting and stopping at the
 * specified rows.  ending at the last row in the table.  Return the
 * specified columns.  Only values with the specified timestamp are
 * returned.
 * 
 * @return scanner id to be used with other scanner procedures
 * 
 * Parameters:
 *  - TableName: name of table
 *  - StartRow: Starting row in table to scan.
 * Send "" (empty string) to start at the first row.
 *  - StopRow: row to stop scanning on. This row is *not* included in the
 * scanner's results
 *  - Columns: columns to scan. If column name is a column family, all
 * columns of the specified column family are returned. It's also possible
 * to pass a regex in the column qualifier.
 *  - Timestamp: timestamp
 *  - Attributes: Scan attributes
 */
func (client *HClient) ScannerOpenWithStopTs(tableName string, startRow string, stopRow string, columns []string, timestamp int64, attributes map[string]string) (ScannerID, error) {
	fmt.Println(tableName, startRow, stopRow, columns, timestamp, attributes)
	return ScannerID(0), nil
}

/**
 * Returns the scanner's current row value and advances to the next
 * row in the table.  When there are no more rows in the table, or a key
 * greater-than-or-equal-to the scanner's specified stopRow is reached,
 * an empty list is returned.
 * 
 * @return a TRowResult containing the current row and a map of the columns to TCells.
 * 
 * @throws IllegalArgument if ScannerID is invalid
 * 
 * @throws NotFound when the scanner reaches the end
 * 
 * Parameters:
 *  - Id: id of a scanner returned by scannerOpen
 */
func (client *HClient) ScannerGet(id ScannerID) ([]TRowResult, error) {
	fmt.Println(id)
	return nil, nil
}

/**
 * Returns, starting at the scanner's current row value nbRows worth of
 * rows and advances to the next row in the table.  When there are no more
 * rows in the table, or a key greater-than-or-equal-to the scanner's
 * specified stopRow is reached,  an empty list is returned.
 * 
 * @return a TRowResult containing the current row and a map of the columns to TCells.
 * 
 * @throws IllegalArgument if ScannerID is invalid
 * 
 * @throws NotFound when the scanner reaches the end
 * 
 * Parameters:
 *  - Id: id of a scanner returned by scannerOpen
 *  - NbRows: number of results to return
 */
func (client *HClient) ScannerGetList(id ScannerID, nbRows int32) ([]TRowResult, error) {
	fmt.Println(id, nbRows)
	return nil, nil
}

/**
 * Closes the server-state associated with an open scanner.
 * 
 * @throws IllegalArgument if ScannerID is invalid
 * 
 * Parameters:
 *  - Id: id of a scanner returned by scannerOpen
 */
func (client *HClient) ScannerClose(id ScannerID) error {
	fmt.Println(id)
	return nil
}

/**
 * Get the row just before the specified one.
 * 
 * @return value for specified row/column
 * 
 * Parameters:
 *  - TableName: name of table
 *  - Row: row key
 *  - Family: column name
 */
func (client *HClient) GetRowOrBefore(tableName string, row string, family string) ([]TCell, error) {
	fmt.Println(tableName, row, family)
	return nil, nil
}

/**
 * Get the regininfo for the specified row. It scans
 * the metatable to find region's start and end keys.
 * 
 * @return value for specified row/column
 * 
 * Parameters:
 *  - Row: row key
 */
func (client *HClient) GetRegionInfo(row string) (*TRegionInfo, error) {
	fmt.Println(row)
	return nil, nil
}
