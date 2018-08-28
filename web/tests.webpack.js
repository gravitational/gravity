var context = require.context('./src/app/', true, /\Test.js$/);
context.keys().forEach(context);
