package server

import "fmt"

func Example_representativePort() {
	fmt.Println(representativePort("tcp://127.0.0.1:1234"))
	fmt.Println(representativePort("http://192.168.1.100:1234"))
	fmt.Println(representativePort("unix:///var/run/neo-server.sock"))
	// Output:
	//   > Local:   http://127.0.0.1:1234
	//   > Network: http://192.168.1.100:1234
	//   > Unix:    /var/run/neo-server.sock
}
