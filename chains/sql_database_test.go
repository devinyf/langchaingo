package chains

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools/sqldatabase"
	"github.com/tmc/langchaingo/tools/sqldatabase/mysql"
)

func TestSQLDatabaseChain_Call(t *testing.T) {
	t.Parallel()
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	llm, err := openai.New()
	require.NoError(t, err)

	// export LANGCHAINGO_TEST_MYSQL=user:p@ssw0rd@tcp(localhost:3306)/test
	mysqlURI := os.Getenv("LANGCHAINGO_TEST_MYSQL")
	if mysqlURI == "" {
		t.Skip("LANGCHAINGO_TEST_MYSQL not set")
	}
	engine, err := mysql.NewMySQL(mysqlURI)
	require.NoError(t, err)

	db, err := sqldatabase.NewSQLDatabase(engine, nil)
	require.NoError(t, err)

	chain := NewSQLDatabaseChain(llm, 5, db)
	input := map[string]interface{}{
		"query":              "How many cards are there?",
		"table_names_to_use": []string{"AllianceAuthority", "AllianceGift", "Card"},
	}
	result, err := chain.Call(context.Background(), input)
	require.NoError(t, err)

	ret, ok := result["result"].(string)
	require.True(t, ok)
	require.NotEmpty(t, ret)

	t.Log(ret)
}

func TestExtractSQLQuery(t *testing.T) {
	t.Parallel()

	// mock single line sql query.
	simpleSQLQuery := "SELECT * FROM example_table;"

	// mock multi line sql query.
	bareMultiLineSQLQuery := `
SELECT
    orders.order_id,
    customers.customer_name,
    orders.order_date
FROM
    orders
INNER JOIN customers ON orders.customer_id = customers.customer_id
WHERE
    orders.order_date >= '2023-01-01'
ORDER BY
    orders.order_date;
`

	// the same multi line sql query above with no indentation,
	// this is the expected result after the extract function.
	bareMultiLineSQLQueryWithNoIndentation := `SELECT
orders.order_id,
customers.customer_name,
orders.order_date
FROM
orders
INNER JOIN customers ON orders.customer_id = customers.customer_id
WHERE
orders.order_date >= '2023-01-01'
ORDER BY
orders.order_date;`

	singleLinePollutedtestCases := polluteSQLSyntaxTestCase(simpleSQLQuery)
	multiLinePollutedtestCases := polluteSQLSyntaxTestCase(bareMultiLineSQLQuery)

	testCases := []extractMultiLineSQLTestCase{
		{simpleSQLQuery, simpleSQLQuery},
		{bareMultiLineSQLQuery, bareMultiLineSQLQueryWithNoIndentation},
	}

	for _, v := range singleLinePollutedtestCases {
		testCases = append(testCases, extractMultiLineSQLTestCase{v, simpleSQLQuery})
	}

	for _, v := range multiLinePollutedtestCases {
		testCases = append(testCases, extractMultiLineSQLTestCase{v, bareMultiLineSQLQueryWithNoIndentation})
	}

	for _, tc := range testCases {
		filteredMultiLineQuery := extractSQLQuery(tc.inputStr)
		require.Equal(t, tc.expected, filteredMultiLineQuery)
	}
}

type extractMultiLineSQLTestCase struct {
	inputStr string
	expected string
}

// different llm model result text format differently.
// this function is used to pollute the sql query syntax text to simulate the different format.
func polluteSQLSyntaxTestCase(sql string) []string {
	// simulate the return output formot-1: contain illusion text above and below
	// some extra text here.
	// SQLQuery SELECT xxx... ;
	// SQLResult: 3 (illusion data)
	// Answer: There are 3 data in the table. (illusion data)
	taintedOutput1 := `
I am a clumsy llm model
I just feel good to put some extra text here.
SQLQuery: %s
SQLResult: 3 (illusion data)
Answer: There are 3 data in the table. (illusion data)
`

	// simulate the return output formot-2: contain illusion text below
	// SELECT xxx... ;
	// SQLResult: 3 (illusion data)
	// Answer: There are 3 data in the table. (illusion data)
	taintedOutput2 := `
%s
SQLResult: 3 (illusion data)
Answer: There are 3 data in the table. (illusion data)
`

	// simulate the return output formot-3: contain markdown symbols
	// ```sql
	// SELECT xxx... ;
	// ```
	// SQLResult: 3 (illusion data)
	// Answer: There are 3 data in the table. (illusion data)
	taintedOutput3 := "```sql\n%s\n```\nSQLResult: 3 (illusion data)\nAnswer: There are 3 data in the table. (illusion data)\n"

	polluteSQL1 := fmt.Sprintf(taintedOutput1, sql)
	polluteSQL2 := fmt.Sprintf(taintedOutput2, sql)
	polluteSQL3 := fmt.Sprintf(taintedOutput3, sql)

	return []string{polluteSQL1, polluteSQL2, polluteSQL3}
}
