var ossContext = require.context('./oss-src/', true, /\Test.js$/);
var entContext = require.context('./src/', true, /\Test.js$/);
ossContext.keys().forEach(ossContext);
entContext.keys().forEach(entContext);