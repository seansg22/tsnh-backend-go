package handler

import "testing"

func TestValidate(t *testing.T) {
	ok := map[string]string{}
	cases := []struct {
		r        request
		needData bool
		valid    bool
	}{
		{request{"mimi_01", "123456", nil}, false, true},
		{request{"ab", "123456", nil}, false, false},
		{request{"Mimi", "123456", nil}, false, false},
		{request{"mimi", "12345", nil}, false, false},
		{request{"mimi", "12345a", nil}, false, false},
		{request{"mimi", "123456", nil}, true, false},
		{request{"mimi", "123456", ok}, true, true},
	}
	for i, c := range cases {
		if got := validate(c.r, c.needData) == ""; got != c.valid {
			t.Errorf("case %d: valid=%v want %v", i, got, c.valid)
		}
	}
}
