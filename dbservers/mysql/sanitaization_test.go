package mysql

import "testing"

func TestQuoteMySQLIdentifier(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal table name",
			input:    "testTable",
			expected: "`testTable`",
		},
		{
			name:     "table name with backticks",
			input:    "test`Table",
			expected: "`test``Table`",
		},
		{
			name:     "table name with SQL injection attempt",
			input:    "; DROP TABLE users;",
			expected: "`; DROP TABLE users;`",
		},
		{
			name:     "table name with comment injection attempt",
			input:    "testTable` --",
			expected: "`testTable`` --`",
		},
		{
			name:     "table name with boolean-based SQL injection attempt",
			input:    "testTable` AND 1=1",
			expected: "`testTable`` AND 1=1`",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := quoteMySQLIdentifier(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}
