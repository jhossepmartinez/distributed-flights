## Integrantes:
    Jhossep Martinez / 202173530-5
    Fernando Xais / 202273551-1
    Gabriela Yáñez / 202273511-2

## Consideraciones

- Cada avion solo tiene 2 asientos, 1A y 1B
- Los clientes RYW se deben ejecutar manualmente cuando estan corriendo el broker, datanodes, coordinador
- Los conflictos por concurrecia se manejan por orden alfabetico

## Como ejecutar

- en vm 1 ejecutar ```make build-vm1```
- en vm 2 ejecutar ```make build-vm2```
- en vm 3 ejecutar ```make build-vm3```
- en vm 4 ejecutar ```make build-vm4```
- en la vm 4 mantener el archivo de prueba flight_updates.csv o reemplazarlo por otro manteniendo el nombre del archivo
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-1```
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-2```
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-3```
