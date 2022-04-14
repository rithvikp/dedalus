package engine

//func TestCoreFDs(t *testing.T) {
//tests := []struct {
//msg   string
//facts []*fact
//attrs []Attribute
//fds   []CoreFD
//}{
//{
//msg: "2 -> 1 FD",
//facts: []*fact{
//{data: []string{"a", "b", "c"}},
//{data: []string{"a", "c", "c"}},
//{data: []string{"b", "b", "c"}},
//{data: []string{"a", "a", "d"}},
//},
//attrs: []Attribute{{index: 0}, {index: 1}, {index: 2}},
//fds:   []CoreFD{},
//},
//}

//for _, tt := range tests {
//tt := tt
//t.Run(tt.msg, func(t *testing.T) {
//got := coreFDs(tt.facts, tt.attrs, "f")
//if !reflect.DeepEqual(tt.fds, got) {
//t.Errorf("Core FDs did not match: got %v\n\n want %v", got, tt.fds)
//}
//})
//}
//}
