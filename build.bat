@echo off
SET version=%1
IF "%version%" == "" (
    SET version=latest
)
SET prefix=%2
IF "%prefix%" == "" (
    SET prefix=dockerlinkpop
)
@echo "Build params : %version%" "%prefix%" 

docker build -f .\docker_application_build --tag %prefix%/tarantula.admin:%version% --build-arg app=admin .
IF %ERRORLEVEL% NEQ 0 ( 
   @echo "build failed, try again"
   goto Clean 
)

docker build -f .\docker_application_build --tag %prefix%/tarantula.cloud:%version% --build-arg app=cloud .
IF %ERRORLEVEL% NEQ 0 ( 
    @echo "build failed, try again"
    goto Clean
)

docker build -f .\docker_application_build --tag %prefix%/tarantula.postoffice:%version% --build-arg app=postoffice .
IF %ERRORLEVEL% NEQ 0 ( 
    @echo "build failed, try again"
    goto Clean
)

docker build -f .\docker_caddy_build --tag %prefix%/tarantula.caddy:%version% .
IF %ERRORLEVEL% NEQ 0 ( 
    @echo "build failed, try again"
    goto Clean
)

:Clean
docker builder prune -af
@echo "deleting build files"
