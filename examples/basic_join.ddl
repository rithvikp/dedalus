out(a,b,c,L1,T) :- in1(a,b,L1,T), in2(b,c,L1,T)
out(a,b,c,L1,S) :- out(a,b,c,L1,T), choose((a,b,c),S)

in1("a","b",L1,0).
in1("f","b",L1,0).
in2("b","c",L1,0).
in2("a","b",L1,0).
