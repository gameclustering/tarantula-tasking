@echo off
SET version=%1
IF "%version%" == "" (
    SET version=latest
)
@echo "Build params : %version%"

docker build -f .\docker_application_build --tag tarantula.admin:%version% --build-arg app=admin .
IF %ERRORLEVEL% NEQ 0 ( 
   @echo "build failed, try again"
   goto Clean 
)

docker build -f .\docker_application_build --tag tarantula.cloud:%version% --build-arg app=cloud .
IF %ERRORLEVEL% NEQ 0 ( 
    @echo "build failed, try again"
    goto Clean
)

docker build -f .\docker_application_build --tag tarantula.postoffice:%version% --build-arg app=postoffice .
IF %ERRORLEVEL% NEQ 0 ( 
    @echo "build failed, try again"
    goto Clean
)

docker build -f .\docker_nginx_build --tag tarantula.nginx:%version% .
IF %ERRORLEVEL% NEQ 0 ( 
    @echo "build failed, try again"
    goto Clean
)

:Clean
docker builder prune -af
@echo "deleting build files"
