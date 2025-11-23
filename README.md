## Integrantes:
    Jhossep Martinez / 202173530-5
    Fernando Xais / 202273551-1
    Gabriela Yáñez / 202273511-2

## Consideraciones

- Todas las imagenes ya estan compiladas
- Cada avion solo tiene 2 asientos, 1A y 1B
- Los clientes RYW se deben ejecutar manualmente cuando estan corriendo el broker, datanodes, coordinador
- Los conflictos por concurrecia se manejan por orden alfabetico
- En el reporte.txt solo se agregan las asignaciones de pistas
- En caso de querer agregar otro archivo de prueba se tiene que volver a compilar el broker

- vm1 es dist013
- vm2 es dist014
- vm3 es dist015
- vm4 es dist016

- vm1 tiene los datanodes
- vm2 tiene al coordinador
- vm3 tiene a los coordinadores de trafico
- vm4 tiene al broker y monotonic clients
- vm4 es donde se ejecuta a los clientes RYW

## Credenciales

**dist013**
- ehe6gqRsS2Fk
- 10.35.168.23

**dist014** 
- KRZ65kfAEmpB
- 10.35.168.24

**dist015**
- aNASDGkYnQ8F
- 10.35.168.25

**dist016**
- jrKU59Umn2TW
- 10.35.168.26

## Como ejecutar

- en vm 1 ejecutar ```make build-vm1```
- en vm 2 ejecutar ```make build-vm2```
- en vm 3 ejecutar ```make build-vm3```
- en vm 4 ejecutar ```make build-vm4```
- (opcional) si se modifica el archivo csv de prueba correr en la vm4: ```sudo make build-broker``` y luego seguir hacer ```make build-vm4```
- en la vm 4 mantener el archivo de prueba flight_updates.csv o reemplazarlo por otro manteniendo el nombre del archivo
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-1```
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-2```
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-3```
- Una vez terminada la lectura del archivo csv, el broker genera el archivo reporte.txt que tiene las asginaciones de pista.
