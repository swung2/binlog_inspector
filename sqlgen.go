package main

import (
	"fmt"
	"strings"

	SQL "github.com/dropbox/godropbox/database/sqlbuilder"
	sqltypes "github.com/dropbox/godropbox/database/sqltypes"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/replication"
	sliceKits "github.com/toolkits/slice"
)

var G_Bytes_Column_Types []string = []string{"blob", "json", "geometry", UNKNOWN_FIELD_TYPE_NAME}

// get column data type name and sqlbuilder column definition
/*
bit
bool
tinyint
smallint
mediumint
int
bigint
decimal

float
double

char
varchar
binary
varbinary

tinytext
text
mediumtext
longtext

tinyblob
blob
mediumblob
longblob

set
enum

date
time
datetime
timestamp
year

geometry
point
linestring
polygon

multipoint
mulitlinestring
mulitpolygon
geometrycollection

json
*/

func GetMysqlDataTypeNameAndSqlColumn(tpDef string, colName string, tp byte, meta uint16) (string, SQL.NonAliasColumn) {
	// for unkown type, defaults to BytesColumn

	//get real string type
	if tp == mysql.MYSQL_TYPE_STRING {
		if meta >= 256 {
			b0 := uint8(meta >> 8)
			if b0&0x30 != 0x30 {
				tp = byte(b0 | 0x30)
			} else {
				tp = b0
			}
		}
	}
	//fmt.Println("column type:", colName, tp)
	switch tp {

	case mysql.MYSQL_TYPE_NULL:
		return UNKNOWN_FIELD_TYPE_NAME, SQL.BytesColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_LONG:
		return "int", SQL.IntColumn(colName, SQL.NotNullable)

	case mysql.MYSQL_TYPE_TINY:
		return "tinyint", SQL.IntColumn(colName, SQL.NotNullable)

	case mysql.MYSQL_TYPE_SHORT:
		return "smallint", SQL.IntColumn(colName, SQL.NotNullable)

	case mysql.MYSQL_TYPE_INT24:
		return "mediumint", SQL.IntColumn(colName, SQL.NotNullable)

	case mysql.MYSQL_TYPE_LONGLONG:
		return "bigint", SQL.IntColumn(colName, SQL.NotNullable)

	case mysql.MYSQL_TYPE_NEWDECIMAL:
		return "decimal", SQL.DoubleColumn(colName, SQL.NotNullable)

	case mysql.MYSQL_TYPE_FLOAT:
		return "float", SQL.DoubleColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_DOUBLE:
		return "double", SQL.DoubleColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_BIT:
		return "bit", SQL.IntColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_TIMESTAMP:
		return "timestamp", SQL.DateTimeColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_TIMESTAMP2:
		return "timestamp", SQL.DateTimeColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_DATETIME:
		return "datetime", SQL.DateTimeColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_DATETIME2:
		return "datetime", SQL.DateTimeColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_TIME:
		return "time", SQL.StrColumn(colName, SQL.UTF8, SQL.UTF8CaseInsensitive, SQL.NotNullable)
	case mysql.MYSQL_TYPE_TIME2:
		return "time", SQL.StrColumn(colName, SQL.UTF8, SQL.UTF8CaseInsensitive, SQL.NotNullable)
	case mysql.MYSQL_TYPE_DATE:
		return "date", SQL.StrColumn(colName, SQL.UTF8, SQL.UTF8CaseInsensitive, SQL.NotNullable)

	case mysql.MYSQL_TYPE_YEAR:
		return "year", SQL.IntColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_ENUM:
		return "enum", SQL.IntColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_SET:
		return "set", SQL.IntColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_BLOB:
		//text is stored as blob
		if strings.Contains(strings.ToLower(tpDef), "text") {
			return "blob", SQL.StrColumn(colName, SQL.UTF8, SQL.UTF8CaseInsensitive, SQL.NotNullable)
		}
		return "blob", SQL.BytesColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_VARCHAR,
		mysql.MYSQL_TYPE_VAR_STRING:

		return "varchar", SQL.StrColumn(colName, SQL.UTF8, SQL.UTF8CaseInsensitive, SQL.NotNullable)
	case mysql.MYSQL_TYPE_STRING:
		return "char", SQL.StrColumn(colName, SQL.UTF8, SQL.UTF8CaseInsensitive, SQL.NotNullable)
	case mysql.MYSQL_TYPE_JSON:
		return "json", SQL.BytesColumn(colName, SQL.NotNullable)
	case mysql.MYSQL_TYPE_GEOMETRY:
		return "geometry", SQL.BytesColumn(colName, SQL.NotNullable)
	default:
		return UNKNOWN_FIELD_TYPE_NAME, SQL.BytesColumn(colName, SQL.NotNullable)
	}
}

func CheckRowMatchColumns(rows []interface{}, colNames []FieldInfo) (int, string) {
	// 1: column added, -1: column dropped, 0: column match
	rLen := len(rows)
	cLen := len(colNames)

	if rLen < cLen {
		return 1, "binlog row data missing some fields(altered table add column), ignore missing fields"
	} else if rLen > cLen {
		return -1, fmt.Sprintf("some table fields missing(altered table drop column), map missing field to %s*", UNKNOWN_FIELD_NAME_PREFIX)
	}
	return 0, ""
}

func GetFieldName(idx int, colNames []FieldInfo) string {
	cLen := len(colNames)
	if idx < cLen {
		return colNames[idx].FieldName
	} else {
		return GetDroppedFieldName(cLen - idx)
	}
}

func GetDroppedFieldName(idx int) string {
	return fmt.Sprintf("%s%d", UNKNOWN_FIELD_NAME_PREFIX, idx)
}

func GetAllFieldNamesWithDroppedFields(rowLen int, colNames []FieldInfo) []FieldInfo {
	if rowLen <= len(colNames) {
		return colNames
	}
	var arr []FieldInfo = make([]FieldInfo, rowLen)
	cnt := copy(arr, colNames)
	for i := cnt; i < rowLen; i++ {
		arr[i] = FieldInfo{FieldName: GetDroppedFieldName(i - cnt), FieldType: UNKNOWN_FIELD_TYPE_NAME}
	}
	return arr
}

func GetSqlFieldsEXpressions(colCnt int, colNames []FieldInfo, tbMap *replication.TableMapEvent) ([]SQL.NonAliasColumn, []string) {
	colDefExps := make([]SQL.NonAliasColumn, colCnt)
	colTypeNames := make([]string, colCnt)
	for i := 0; i < colCnt; i++ {
		typeName, colDef := GetMysqlDataTypeNameAndSqlColumn(colNames[i].FieldType, colNames[i].FieldName, tbMap.ColumnType[i], tbMap.ColumnMeta[i])
		colDefExps[i] = colDef
		colTypeNames[i] = typeName
	}
	return colDefExps, colTypeNames
}

func GenEqualConditions(row []interface{}, colDefs []SQL.NonAliasColumn, uniKey []int, ifMinImage bool) []SQL.BoolExpression {
	if ifMinImage && len(uniKey) > 0 {
		expArrs := make([]SQL.BoolExpression, len(uniKey))
		for k, idx := range uniKey {
			expArrs[k] = SQL.EqL(colDefs[idx], row[idx])
		}
		return expArrs
	}
	expArrs := make([]SQL.BoolExpression, len(row))
	for i, v := range row {
		expArrs[i] = SQL.EqL(colDefs[i], v)
	}
	return expArrs
}

func ConvertRowToExpressRow(row []interface{}) []SQL.Expression {

	valueInserted := make([]SQL.Expression, len(row))
	for i, val := range row {
		vExp := SQL.Literal(val)
		valueInserted[i] = vExp
	}
	return valueInserted
}

func GenInsertSqlForRows(rows [][]interface{}, insertSql SQL.InsertStatement, schema string, ifprefixDb bool) (string, error) {

	for _, row := range rows {
		valuesInserted := ConvertRowToExpressRow(row)
		insertSql.Add(valuesInserted...)
	}
	if !ifprefixDb {
		schema = ""
	}
	return insertSql.String(schema)

}

func GenInsertSqlsForOneRowsEvent(rEv *replication.RowsEvent, colDefs []SQL.NonAliasColumn, rowsPerSql int, ifRollback bool, ifprefixDb bool) []string {
	rowCnt := len(rEv.Rows)
	schema := string(rEv.Table.Schema)
	table := string(rEv.Table.Table)
	columnCount := len(colDefs)
	var sqlArr []string
	var sqlType string
	if ifRollback {
		sqlType = "insert_for_delete_rollback"
	} else {
		sqlType = "insert"
	}
	var insertSql SQL.InsertStatement
	var oneSql string
	var err error
	var i int
	var endIndex int
	for i = 0; i < rowCnt; i += rowsPerSql {
		insertSql = SQL.NewTable(table, colDefs[:columnCount-2]...).Insert(colDefs[:columnCount-2]...)
		endIndex = GetMinValue(rowCnt, i+rowsPerSql)
		oneSql, err = GenInsertSqlForRows(rEv.Rows[i:endIndex][:columnCount-2], insertSql, schema, ifprefixDb)
		if err != nil {
			PrintGenSqlError(err, rEv.Rows[i:endIndex][:columnCount-2], sqlType, schema, table)
			continue
		} else {
			sqlArr = append(sqlArr, oneSql)
		}

	}

	if endIndex < rowCnt {
		insertSql = SQL.NewTable(table, colDefs...).Insert(colDefs...)
		oneSql, err = GenInsertSqlForRows(rEv.Rows[endIndex:rowCnt][:columnCount-2], insertSql, schema, ifprefixDb)
		if err != nil {
			PrintGenSqlError(err, rEv.Rows[endIndex:rowCnt][:columnCount-2], sqlType, schema, table)
		} else {
			sqlArr = append(sqlArr, oneSql)
		}
	}
	//fmt.Println("one insert sqlArr", sqlArr)
	return sqlArr

}

func GenDeleteSqlsForOneRowsEventRollbackInsert(rEv *replication.RowsEvent, colDefs []SQL.NonAliasColumn, uniKey []int, ifMinImage bool, ifprefixDb bool) []string {
	return GenDeleteSqlsForOneRowsEvent(rEv, colDefs, uniKey, ifMinImage, true, ifprefixDb)
}

func GenDeleteSqlsForOneRowsEvent(rEv *replication.RowsEvent, colDefs []SQL.NonAliasColumn, uniKey []int, ifMinImage bool, ifRollback bool, ifprefixDb bool) []string {
	rowCnt := len(rEv.Rows)
	sqlArr := make([]string, rowCnt)
	//var sqlArr []string
	schema := string(rEv.Table.Schema)
	table := string(rEv.Table.Table)
	columnCount := len(colDefs)
	schemaInSql := schema
	if !ifprefixDb {
		schemaInSql = ""
	}

	var sqlType string
	if ifRollback {
		sqlType = "delete_for_insert_rollback"
	} else {
		sqlType = "delete"
	}
	for i, row := range rEv.Rows {
		whereCond := GenEqualConditions(row[:columnCount-2], colDefs, uniKey, ifMinImage)

		sql, err := SQL.NewTable(table, colDefs...).Delete().Where(SQL.And(whereCond...)).String(schemaInSql)
		if err != nil {
			PrintGenSqlError(err, [][]interface{}{row[:columnCount-2]}, sqlType, schema, table)
			continue
		}
		sqlArr[i] = sql
		//sqlArr = append(sqlArr, sql)
	}
	return sqlArr
}

func GenInsertSqlsForOneRowsEventRollbackDelete(rEv *replication.RowsEvent, colDefs []SQL.NonAliasColumn, rowsPerSql int, ifprefixDb bool) []string {
	return GenInsertSqlsForOneRowsEvent(rEv, colDefs, rowsPerSql, true, ifprefixDb)
}

func GenUpdateSetPart(colsTypeNameFromMysql []string, colTypeNames []string, updateSql SQL.UpdateStatement, colDefs []SQL.NonAliasColumn, rowAfter []interface{}, rowBefore []interface{}, ifMinImage bool) SQL.UpdateStatement {

	ifUpdateCol := false
	for i, v := range rowAfter {
		ifUpdateCol = false
		//fmt.Printf("type: %s\nbefore: %v\nafter: %v\n", colTypeNames[i], rowBefore[i], v)

		if ifMinImage {
			// text is stored as blob in binlog
			if sliceKits.ContainsString(G_Bytes_Column_Types, colTypeNames[i]) && !strings.Contains(strings.ToLower(colsTypeNameFromMysql[i]), "text") {
				aArr, aOk := v.([]byte)
				bArr, bOk := rowBefore[i].([]byte)
				if aOk && bOk {
					if CompareEquelByteSlice(aArr, bArr) {
						//fmt.Println("bytes compare equal")
						ifUpdateCol = false
					} else {
						ifUpdateCol = true
						//fmt.Println("bytes compare unequal")
					}
				} else {
					//fmt.Println("error to convert to []byte")
					//should update the column
					ifUpdateCol = true
				}

			} else {
				if v == rowBefore[i] {
					//fmt.Println("compare equal")
					ifUpdateCol = false
				} else {
					//fmt.Println("compare unequal")
					ifUpdateCol = true
				}
			}
		} else {
			ifUpdateCol = true
		}

		if ifUpdateCol {
			updateSql.Set(colDefs[i], SQL.Literal(v))
		}
	}
	return updateSql

}

func GenUpdateSqlsForOneRowsEvent(colsTypeNameFromMysql []string, colsTypeName []string, rEv *replication.RowsEvent, colDefs []SQL.NonAliasColumn, uniKey []int, ifMinImage bool, ifRollback bool, ifprefixDb bool) []string {
	//colsTypeNameFromMysql: for text type, which is stored as blob
	rowCnt := len(rEv.Rows)
	schema := string(rEv.Table.Schema)
	table := string(rEv.Table.Table)
	schemaInSql := schema
	columnCount := len(colDefs)
	if !ifprefixDb {
		schemaInSql = ""
	}
	var sqlArr []string
	var (
		sql       string
		err       error
		sqlType   string
		wherePart []SQL.BoolExpression
	)
	if ifRollback {
		sqlType = "update_for_update_rollback"
	} else {
		sqlType = "update"
	}
	for i := 0; i < rowCnt; i += 2 {
		delete_val_before := 0
		delete_val_after := 0
		fmt.Printf("%v\n", colDefs[columnCount-1].Name())
		if colDefs[columnCount-1].Name() == "__deleted" {
			var delete_val_b sqltypes.Value
			var delete_val_a sqltypes.Value
			delete_val_b, err = sqltypes.BuildValue(rEv.Rows[i][columnCount-1])
			if err != nil {
				fmt.Printf("\t sqltypes.BuildValue failed %v\n", err)
				continue
			}
			err = sqltypes.ConvertAssign(delete_val_b, &delete_val_before)
			if err != nil {
				fmt.Printf("\tconvert %v to uint64 failed %v\n", rEv.Rows[i][columnCount-1], err)
				continue
			}
			delete_val_a, err = sqltypes.BuildValue(rEv.Rows[i+1][columnCount-1])
			if err != nil {
				fmt.Printf("\t sqltypes.BuildValue failed %v\n", err)
				continue
			}
			err = sqltypes.ConvertAssign(delete_val_a, &delete_val_after)
			if err != nil {
				fmt.Printf("\tconvert %v to uint64 failed %v\n", rEv.Rows[i+1][columnCount-1], err)
				continue
			}
		}
		fmt.Printf("%v<->%v\n", delete_val_before, delete_val_after)
		if delete_val_before == 0 && delete_val_after == 1 {
			insertSql := SQL.NewTable(table, colDefs[:columnCount-2]...).Insert(colDefs[:columnCount-2]...)
			var oneSql string
			oneSql, err = GenInsertSqlForRows([][]interface{}{rEv.Rows[i][:columnCount-2]}, insertSql, schema, ifprefixDb)
			if err != nil {
				PrintGenSqlError(err, [][]interface{}{rEv.Rows[i][:columnCount-2]}, sqlType, schema, table)
				continue
			}
			sql = oneSql

		} else if delete_val_before == 1 && delete_val_after == 0 {
			whereCond := GenEqualConditions(rEv.Rows[i][:columnCount-2], colDefs[:columnCount-2], uniKey, ifMinImage)

			sql, err = SQL.NewTable(table, colDefs[:columnCount-2]...).Delete().Where(SQL.And(whereCond...)).String(schemaInSql)
			if err != nil {
				PrintGenSqlError(err, [][]interface{}{rEv.Rows[i][:columnCount-2]}, sqlType, schema, table)
				continue
			}

		} else {
			upSql := SQL.NewTable(table, colDefs...).Update()
			if ifRollback {
				upSql = GenUpdateSetPart(colsTypeNameFromMysql, colsTypeName, upSql, colDefs, rEv.Rows[i][:columnCount-2], rEv.Rows[i+1], ifMinImage)
				wherePart = GenEqualConditions(rEv.Rows[i+1][:columnCount-2], colDefs, uniKey, ifMinImage)
			} else {
				upSql = GenUpdateSetPart(colsTypeNameFromMysql, colsTypeName, upSql, colDefs, rEv.Rows[i+1][:columnCount-2], rEv.Rows[i], ifMinImage)
				wherePart = GenEqualConditions(rEv.Rows[i][:columnCount-2], colDefs, uniKey, ifMinImage)
			}

			upSql.Where(SQL.And(wherePart...))
			sql, err = upSql.String(schemaInSql)
		}
		if err != nil {
			PrintGenSqlError(err, [][]interface{}{}, sqlType, schema, table)
			fmt.Printf("\toriginally, row data before updated: %v\n\t row data after updated: %v\n", rEv.Rows[i], rEv.Rows[i+1])

		} else {
			sqlArr = append(sqlArr, sql)
		}

	}
	//fmt.Println(sqlArr)
	return sqlArr

}

func PrintGenSqlError(err error, row [][]interface{}, sqlType, schema, table string) {
	fmt.Printf("Fail to generate %s sql for %s.%s.\n\terror: %s\n\trows data:%v\n", sqlType, schema, table, err, row)
}
