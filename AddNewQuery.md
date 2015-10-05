To add a new query, you would have to:

1. In `configuration/configuration.go` add a new var for the new flag. If the flag isn't a boolean, add also a varSet. Add them below `// flag variables`
2. If not boolean, add the detection switch at `func setVisitedFlags(f *flag.Flag)`
3. Add the actual flag inside `init()`
4. Inside the switch of `varSet`s add your new query detection (`// Determine operation`). If your query takes as argument a string, let it be the query string. If it takes an int, assign it to QParams.Kappa. If you need both the query and a query string, we need to add a new field to QueryParams.
5. Add the constant of its operation (QUERY_[OPERATION]) at `// Available query types`
6. Inside `database/queries.go` add a function of type `func(qp conf.QueryParams) ([]byte, err)` that implements your query. Your result should not end with a newline.
7. Inside function `RunQuery` add a detection for your query type.
8. In `configuration/configuration.go` add help text for your query (function `PrintHelp`)
