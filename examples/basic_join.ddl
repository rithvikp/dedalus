out(a,b,c,l,t) :- in1(a,b,l,t), in2(b,c,l,t)
out2(max<a>,b,c,l,t') :- out(a,b,c,l,t), choose((a,b,c),t')

in1("1","2",L1,0).
in1("5","2",L1,0).
in2("2","3",L1,0).
in2("1","2",L1,0).
