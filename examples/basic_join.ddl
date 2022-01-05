out(a,b,c,L1,T) :- in1(a,b,L1,T), in2(b,c,L1,T)
out2(a,b,c,L1,S) :- out(a,b,c,L1,T), choose((a,b,c),S)

in1("1","2",L1,0).
in1("5","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).
