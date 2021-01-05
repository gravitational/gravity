const req = require.context('../', true, /\.story.js$/)
req.keys().forEach(req)