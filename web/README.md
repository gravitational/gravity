## Web Client

#### To build using docker:

```
$ make
```

#### To build using npm:

install nodejs >= 8.0.0

```
$ npm install
$ npm run build
```

#### To run unit-tests:

```
$ npm run tdd
```

#### To run a local development server:

```
$ npm run start -- --proxy=https://host:port
```


For example, if you have a cluster `https://mycluster.example.com` and want to
use it as a backend for your local WEB development, you can start a
dev server with the following:
```
$ npm run start -- --proxy=https://mycluster.example.com
```


Then open `https://localhost:8081/web`

