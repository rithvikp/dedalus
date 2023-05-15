# Dedalus
An interpreter and analyzer for [Dedalus](https://dsf.berkeley.edu/papers/datalog2011-dedalus.pdf), a Datalog-based formalism for distributed systems. This repository contains both a barebones interpreter for Dedalus and a basic analysis toolkit, including functional dependency tracing. The interpreter is limited, and the significantly improved version based on [Hydroflow](https://github.com/hydro-project/hydroflow) should be used instead.

#### Notes
- PascalCased relations are automatically persisted.
- Stratification is not implemented, so aggregation and negation only work in certain circumstances.
