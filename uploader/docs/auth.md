# Authorization

The service is supposed to be used only through the web interface.

Authorization is performed using the configurable `<auth_cookie_name>` cookie:
* The user interface, along with the request, sends the `<auth_cookie_name>` cookie to the microservice
* microservice forwards cookie to YTsaurus proxy
* YTsaurus proxy exchanges the cookie for the user and checks @acl
