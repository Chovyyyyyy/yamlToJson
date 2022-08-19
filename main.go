package main

import (
	"fmt"
)

type Person struct {
	Name string `json:"name"` // Affects YAML field names too.
	Age  int    `json:"age"`
}

func main() {
	p := Person{"John", 30}
	y, err := Marshal(p)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}
	fmt.Println(string(y))
	fmt.Println("!!!!!!!!!")
	/* Output:
	name: John
	age: 30
	*/
	j2, err := YAMLToJSON(y)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}
	fmt.Println(string(j2))
	/* Output:
	{"age":30,"name":"John"}
	*/
}
