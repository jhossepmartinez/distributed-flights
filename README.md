## Integrantes:
    Jhossep Martinez / 202173530-5
    Fernando Xais / 202273551-1
    Gabriela Yáñez / 202273511-2

## Consideraciones

- Cada avion solo tiene 2 asientos, 1A y 1B
- Los clientes RYW se deben ejecutar manualmente cuando estan corriendo el broker, datanodes, coordinador
- Los conflictos por concurrecia se manejan por orden alfabetico

## Como ejecutar

- en vm 1 ejecutar ```docker compose -f docker-compose.vm1.yaml up```
- en vm 2 ejecutar ```docker compose -f docker-compose.vm2.yaml up```
- en vm 3 ejecutar ```docker compose -f docker-compose.vm3.yaml up```
- en vm 4 ejecutar ```docker compose -f docker-compose.vm4.yaml up```
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-1```
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-2```
- Una vez estan corriedo todos, desde la vm4 ejecutar ```make run-ryw-3```
