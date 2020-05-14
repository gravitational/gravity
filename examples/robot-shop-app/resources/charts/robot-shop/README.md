# Stan's Robot Shop

Use this helm chart to customise your install of Stan's Robot Shop.


## Payment Gateway

By default the `payment` service uses https://www.paypal.com as the pseudo payment provider. The code only does a HTTP GET against this url. You can use a different url by setting within the value.yaml file:

```shell
$ gravity app install  ... --set payment.gateway=https://foobar.com ...
```


