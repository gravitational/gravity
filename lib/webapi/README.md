This is a description of HTTP response headers that we set in our API handlers. These headers help with preventing caching of HTTP traffic and with XXS/CSRF attack mitigation. 

Below table shows a list of headers per request type. This list is a subject to change based on the new findings and recommendations.

||index.html|REST api|
|--- |--- |---
|Cache-Control: no-store; no-cache;must-revalidate| y| y
|Strict-Transport-Security: max-age=31536000; includeSubDomains| y|
|Pragma: no-cache|y
|Expires: 0|y
|X-Frame-Options:SAMEORIGIN|y
|X-XSS-Protection:1; mode=block|y
|X-Content-Type-Options", "nosniff"|y
|content-security-policy:script-src 'self';style-src 'self' 'unsafe-inline';object-src 'none';img-src 'self' data: blob:;child-src 'self'|y


# Caching headers

## Cache-Control

This header is used to specify directives for caches along the request/response chain. Such cache directives are unidirectional in that the presence of a directive in a request does not imply that the same directive is to be given in the response.

```no-store``` - indicates that a cache MUST NOT store any part of either this request or any response to it.

```no-cache``` - indicates that a cache MUST NOT use a stored response to satisfy the request without successful validation on the origin server.

```must-revalidate``` - once the cache expires, refuse to return stale responses to the user even if they say that stale responses are acceptable.

Both, request and response may have these flags instructing other sides on caching. 

```Unless specifically constrained by a cache-control (section 14.9) directive, a caching system MAY always store a successful response (see section 13.8) as a cache entry, MAY return it without validation if it is fresh, and MAY return it after successful validation. If there is neither a cache validator nor an explicit expiration time associated with a response, we do not expect it to be cached, but certain caches MAY violate this expectation (for example, when little or no network connectivity is available). A client can usually detect that such a response was taken from a cache by comparing the Date header to the current time.```

https://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html#sec13.4

https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9.4

https://tools.ietf.org/html/rfc7234#page-21

https://stackoverflow.com/questions/1046966/whats-the-difference-between-cache-control-max-age-0-and-no-cache


## Pragma

This is a HTTP 1.0 directive that was retained in HTTP 1.1 for backward compatibility. When specified in HTTP requests, this directive instructs proxies in the path not to cache the request.

```no-cache``` - the same as Cache-Control no-store.

Developers often misuse it by adding it to the server response when page is served to the browser (facebook is an example). We are are going to misuse it as well since it does no harm including it.

https://tools.ietf.org/html/rfc7234#section-5.4

## Expires

This is yet another HTTP 1.0 directive that was retained for backward compatibility. This directive tells the browser when a page is set to expire. Once the page expires, the browser does not display the page to the user. Instead it shows a message like “Warning: Page has expired”.

```0``` - a cache recipient MUST interpret invalid date formats, especially the value "0", as representing a time in the past (i.e., "already expired").

https://tools.ietf.org/html/rfc7234#section-5.3

# XXS Attack Protection Headers

## X-Frame-Options

This header can be used to indicate whether or not a browser should be allowed to render a page in a ```<frame>```, ```<iframe>``` or ```<object>``` . Sites can use this to avoid clickjacking attacks, by ensuring that their content is not embedded into other sites.

```SAMEORIGIN``` - the page can only be displayed in a frame on the same origin as the page itself.

https://tools.ietf.org/html/rfc7034


## X-XSS-Protection

This header is a feature of Internet Explorer, Chrome and Safari that stops pages from loading when they detect reflected cross-site scripting (XSS) attacks. Although these protections are largely unnecessary in modern browsers when sites implement a strong Content-Security-Policy that disables the use of inline JavaScript ('unsafe-inline'), they can still provide protections for users of older web browsers that don't yet support CSP.


``` :1; mode=block ``` - enables XSS filtering. Rather than sanitizing the page, the browser will prevent rendering of the page if an attack is detected.

https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-XSS-Protection


## X-Content-Type-Options

This header is a marker used by the server to indicate that the MIME types advertised in the Content-Type headers should not be changed and be followed. This allows to opt-out of MIME type sniffing, or, in other words, it is a way to say that the webmasters knew what they were doing.

```nosniff``` - blocks a request if the requested type is __style__ and the MIME type is not "text/css", or __script__ and the MIME type is not a javascript type.

https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Content-Type-Options


## Strict-Transport-Security

This header is an opt-in security enhancement that is specified by a web application through the use of a special response header. Once a supported browser receives this header that browser will prevent any communications from being sent over HTTP to the specified domain and will instead send all communications over HTTPS. It also prevents HTTPS click through prompts on browsers.The specification has been released and published end of 2012 as RFC 6797.

```max-age=31536000; includeSubDomains ``` - tells a browser to enable HSTS for that exact domain or subdomain, and to remember it for a given number of seconds (1 year)

https://https.cio.gov/hsts/


## Content-Security-Policy

This header allows web site administrators to control resources the user agent is allowed to load for a given page. With a few exceptions, policies mostly involve specifying server origins and script endpoints. This helps guard against cross-site scripting attacks (XSS).

|Directive|Meaning|
|--- |--- |
|```script-src 'self'```|same origin javascript files|
|```style-src 'self' 'unsafe-inline'```|same origin styles or inline styles|
|```object-src 'none'```|no embedded objects|
|```img-src 'self' data: blob:```|same origin images or images with data and blob URIs|
|```child-src 'self'```|same origin sources for browsing contexts loaded using elements such as ```webworker``` and ```<iframe>```|
