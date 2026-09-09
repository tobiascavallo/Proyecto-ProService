# ProServicios

Aplicación web que funciona como **directorio público de trabajadores independientes**. Los trabajadores publican su perfil profesional (biografía, especialidad, portfolio de fotos, disponibilidad y datos de contacto) y los clientes los buscan, los comparan por calificación y los contactan **de forma directa**.

El sistema **no intermedia la contratación**: no gestiona presupuestos, ni pagos, ni agenda de turnos. Su valor está en que el cliente pueda elegir con fundamentos, viendo reseñas reales de otros clientes. El contacto sucede por fuera de la plataforma (teléfono, email, redes).

---

## Actores y permisos

| Actor             | Cómo entra                                  | Qué puede hacer                                                                                                                                          |
| ----------------- | ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Invitado**      | Sin autenticación                           | Buscar y filtrar trabajadores, ver perfiles completos, ver datos de contacto                                                                             |
| **Cliente**       | Google OAuth                                | Todo lo del invitado + calificar, comentar y marcar favoritos                                                                                            |
| **Trabajador**    | Google OAuth                                | Todo lo del invitado + marcar favoritos + gestionar su propio perfil profesional. **No puede calificar ni comentar** a otros trabajadores ni a sí mismo. |
| **Administrador** | Google OAuth (rol asignado a mano en la DB) | Aprobar o rechazar especialidades pendientes, dar de baja perfiles y reseñas                                                                             |

**Regla de oro de autorización:** el rol **nunca viaja desde el frontend**. Siempre se lee del JWT en el backend, dentro del middleware o del handler (`c.Get("role")`). El frontend puede decodificar el token localmente para decidir qué botones mostrar, pero eso es UI, nunca autorización.

**Segunda regla:** un trabajador solo puede modificar **su propio** perfil. La comparación se hace entre el `user_id` del JWT y el `user_id` del perfil, siempre en la capa de Service, nunca confiando en un ID que venga en el body.

---

## Stack Tecnológico

- **Lenguaje:** Go 1.22+
- **Framework HTTP:** Gin (`github.com/gin-gonic/gin`)
- **Base de datos:** MongoDB (driver oficial `go.mongodb.org/mongo-driver/v2/mongo`)
- **Autenticación:** Google OAuth como único método de identidad + JWT propio (access + refresh) emitido por el backend
- **Entornos:** el servidor se levanta en HTTP plano durante todo el desarrollo local. HTTPS queda estrictamente reservado a la configuración del servidor en producción.

> **No hay contraseñas en el sistema.** Al ser toda la identidad delegada a Google, no se guardan hashes ni se usa bcrypt. Si en algún momento aparece un campo de password en un modelo, es un error de diseño.

### Flujo de autenticación

1. El frontend abre el login de Google y obtiene un **ID token**.
2. Lo envía a `POST /api/v1/auth/google`.
3. El backend **valida el ID token contra Google** (nunca confía en el token sin verificar firma, `aud` y expiración).
4. Busca el usuario por `google_id`. Si no existe, lo crea con rol `client`.
5. Emite un **access token** propio (corto) y un **refresh token** (largo). De acá en adelante, todas las rutas protegidas usan el JWT propio, no el de Google.

Convertirse en trabajador es un paso posterior: un usuario ya autenticado crea su perfil profesional y ahí su rol pasa a `worker`.

---

## Arquitectura y estructura de carpetas

Flujo de dependencias estricto: **Handler → Service → Repository**. Una capa nunca conoce a la de arriba.

```
backend/
├── main.go            # wiring de dependencias y registro de rutas
├── config/            # carga de variables de entorno
├── handlers/          # capa HTTP: binding de DTOs, llamada al service, respuesta
├── services/          # lógica de negocio pura (no sabe de HTTP ni de Mongo)
├── repositories/      # única capa autorizada a ejecutar consultas MongoDB
├── models/            # structs de las entidades con tags bson
├── dtos/              # structs de request y response + funciones de mapeo
├── middleware/        # auth JWT, RequireRole, logger, recovery, CORS
└── utils/             # JWT, validaciones compartidas, helpers transversales
```

**Interfaces e inyección de dependencias:** cada capa **define la interfaz de la capa que consume**. El handler declara la interfaz del service que necesita; el service declara la interfaz del repository. Los constructores reciben esas interfaces (`NewWorkerHandler(service WorkerService)`), nunca structs concretos. Así cada capa se puede testear con un mock sin tocar las demás.

---

## Modelo de datos

Cinco colecciones. Nombres de campos en `snake_case` para bson, `camelCase` para json.

### `users` — identidad

`_id`, `google_id` (único), `email`, `name`, `picture_url`, `role` (`client` | `worker` | `admin`), `created_at`

### `workers` — perfil profesional

`_id`, `user_id` (ref a users, único), `bio`, `specialty_ids[]`, `phone`, `contact_email`, `social_links`, `availability_status` (`available` | `busy` | `unavailable`), `availability_schedule`, `photos[]`, `average_rating`, `review_count`, `active`, `created_at`

### `specialties` — catálogo

`_id`, `name`, `slug`, `status` (`approved` | `pending`), `created_by`, `created_at`

### `reviews` — reseñas

`_id`, `worker_id`, `client_id`, `rating` (1 a 5), `comment`, `created_at`

### `favorites` — favoritos

`_id`, `user_id`, `worker_id`, `created_at`

### Decisión de modelado: reseñas desnormalizadas

Las reseñas viven en su **propia colección**, pero `average_rating` y `review_count` se guardan **desnormalizados en el documento del trabajador**.

El motivo: ordenar los resultados de búsqueda por calificación es una funcionalidad central del producto. Si el promedio se calculara con un `$lookup` + agregación en cada búsqueda, el costo crecería con cada reseña del sistema y el requerimiento de respuesta en menos de 3 segundos se caería. Con el promedio ya materializado, la búsqueda es un `find` + `sort` sobre un índice.

**Contrapartida, y es la regla más importante de este modelo:** cada vez que se crea, edita o elimina una reseña, el service **debe recalcular y persistir** `average_rating` y `review_count` del trabajador en la misma operación. MongoDB standalone no da transacciones, así que el orden es: primero insertar la reseña, después actualizar el agregado. Si el update del agregado falla, se loguea el error de forma visible — el dato queda temporalmente desfasado, pero nunca se pierde la reseña.

### Índices requeridos

- `users.google_id` — único
- `workers.user_id` — único
- `workers.specialty_ids` + `average_rating` — para búsqueda filtrada y ordenada
- `specialties.slug` — único
- `reviews.worker_id` — para listar reseñas de un perfil
- `reviews` sobre (`worker_id`, `client_id`) — **único**: un cliente deja una sola reseña por trabajador. Si quiere cambiar su opinión, edita la que ya tiene.
- `favorites` sobre (`user_id`, `worker_id`) — único

---

## Reglas de negocio (V1)

1. **Directorio público.** Buscar, ver perfiles y ver datos de contacto **no requieren autenticación**. Estas rutas van fuera del middleware de auth.
2. **Autenticación solo para escribir.** Calificar, comentar, marcar favoritos y gestionar el perfil propio exigen JWT válido.
3. **Solo el rol `client` puede reseñar.** Un trabajador no puede calificar ni comentar, ni a otros trabajadores ni a sí mismo — evita autoreseñas y sabotaje a la competencia. Se valida en el Service leyendo el rol del JWT: si el rol no es `client`, se devuelve `ErrForbidden`. El administrador tampoco reseña; su rol es moderar.
4. **Una reseña por cliente por trabajador.** Garantizado por el índice único. Si el cliente quiere cambiar su opinión, edita la reseña existente en vez de crear otra. El sistema no verifica que el servicio haya ocurrido realmente, porque no gestiona contrataciones ni pagos: es una decisión consciente del MVP.
5. **Especialidad "otra".** Si el trabajador no encuentra su especialidad, escribe una nueva. Se guarda con `status: pending` y **no aparece en los filtros públicos** hasta que un administrador la aprueba. El trabajador queda igualmente asociado a ella desde el momento de la carga.
6. **Ordenamiento por defecto.** Los resultados de búsqueda salen ordenados por `average_rating` descendente.
7. **Disponibilidad.** El trabajador declara manualmente su estado (`available` / `busy` / `unavailable`) y su horario. El sistema no lo calcula ni lo cambia solo.
8. **Bajas lógicas.** Los perfiles no se borran físicamente: se marcan con `active: false` y dejan de aparecer en las búsquedas.

---

## Alcance: V1 vs V2

**V1 (lo único que se implementa ahora):** registro y perfil de trabajador, catálogo de especialidades con aprobación, reseñas con estrellas y comentario, búsqueda y filtro por especialidad, ordenamiento por calificación, favoritos, disponibilidad, login con Google.

**V2 — NO IMPLEMENTAR. Ni siquiera dejar stubs, structs, endpoints comentados o TODOs:**

- Actor **Empresa Anunciante** y todo el módulo de publicidad
- Generación de biografías con IA y resumen automático de reseñas (Gemini / OpenAI)
- Pasarela de pagos, suscripciones premium (MercadoPago / Stripe)
- Cualquier gestión de cobros. **La V1 es 100% gratuita y el sistema no maneja dinero en ninguna forma.**

Si una tarea parece requerir algo de esta lista, frená y preguntá antes de escribir código.

---

## Manejo de errores

Los services devuelven **errores de dominio tipados**, definidos como variables centralizadas. Los handlers son los únicos que traducen esos errores a HTTP. Un service nunca devuelve un código de estado, y un handler nunca decide reglas de negocio.

| Error de dominio  | HTTP | Cuándo                                                                                                 |
| ----------------- | ---- | ------------------------------------------------------------------------------------------------------ |
| `ErrValidation`   | 400  | Datos mal formados o fuera de rango (ej. rating 7)                                                     |
| `ErrUnauthorized` | 401  | Falta el token o es inválido / expiró                                                                  |
| `ErrForbidden`    | 403  | Token válido pero el rol o la propiedad del recurso no alcanzan (ej. un trabajador intentando reseñar) |
| `ErrNotFound`     | 404  | El recurso no existe                                                                                   |
| `ErrConflict`     | 409  | Violación de unicidad (reseña duplicada, favorito repetido)                                            |
| cualquier otro    | 500  | Error inesperado                                                                                       |

**Formato único de respuesta de error**, en todos los endpoints sin excepción:

```json
{ "error": { "code": "NOT_FOUND", "message": "worker not found" } }
```

Los errores 500 **nunca** exponen el error interno de Mongo o del driver al cliente: se loguea completo del lado del servidor y se responde un mensaje genérico.

---

## Configuraciones críticas

- **CORS:** middleware configurado en el router para permitir la comunicación con el frontend. Origen desde variable de entorno, no hardcodeado.
- **Variables de entorno:** archivo `.env` cargado con `godotenv`. Contiene puerto, URI de Mongo, nombre de la base, JWT secret y Google Client ID. **Nunca hardcodear credenciales.** El `.env` va en `.gitignore` y se versiona un `.env.example` sin valores reales.
- **Middlewares obligatorios:** recovery (evitar que un panic tire el servidor), logger de peticiones, CORS, y validación de JWT en rutas protegidas.
- **Timeouts:** todo acceso a MongoDB en los repositories usa `context.WithTimeout`. Sin excepción — sin esto, un problema de red cuelga la aplicación entera.

---

## Convenciones de código

**Idioma:** código, variables, funciones y mensajes de error en **inglés**. Los comentarios, en **español**.

**Comentarios:** cada método nuevo lleva un comentario breve que explique _qué hace y por qué_, no solo qué. El "por qué" es lo que no se deduce leyendo el código.

```go
// GetWorkers devuelve los trabajadores filtrados por especialidad, ordenados por
// calificación descendente. Usa el promedio desnormalizado en vez de calcularlo
// con una agregación porque este endpoint es el más consultado del sistema y
// tiene que responder en menos de 3 segundos.
```

**DTOs:** el modelo **nunca** se expone directo en la API. Toda entrada y toda salida pasa por un DTO explícito en `/dtos`, con su función de mapeo. Esto evita que un campo interno se filtre sin querer y deja el contrato de la API independiente del esquema de la base.

**Errores:** manejo explícito con `if err != nil`. Nunca silenciar un error ni ignorarlo con `_`.

**Funciones:** cortas, con nombre autodescriptivo. Si una función necesita un comentario para explicar _qué_ hace, probablemente hay que dividirla.

**Tolerancia a datos incompletos:** todo código debe soportar documentos con campos ausentes (string vacío, ObjectID zero, array nil) sin entrar en panic.

---

## Verificación obligatoria

Después de **cada** cambio en el backend:

```bash
go build ./... && go vet ./...
```

**No avanzar al siguiente paso si algo no compila.** Si un cambio rompe la compilación, se arregla antes de seguir — nunca se acumulan errores para resolverlos al final.

---

## Cómo trabajar en este proyecto

Este proyecto lo desarrolla un estudiante de ingeniería con base sólida en backend y arquitecturas en capas.

- Si un cambio toca el modelo de datos o afecta varias capas a la vez, **contá el plan completo y esperá confirmación** antes de ejecutar.
- Cuando haya más de un enfoque razonable, planteá las opciones con sus trade-offs en vez de elegir en silencio.
- Trabajá de a una entidad completa por vez (model → dto → repository → service → handler → ruta), verificando que compile antes de pasar a la siguiente.
- No agregues dependencias nuevas sin avisar y justificar para qué.

---

## Mantenimiento de este archivo

Si al agregar contenido este archivo supera las **400 líneas**, no lo agregues sin avisar. Frená y proponé partirlo en un índice corto más archivos de referencia que se lean bajo demanda:

```
backend/
├── CLAUDE.md              # solo reglas siempre activas
└── docs/
    ├── modelo-datos.md    # entidades, índices, decisiones de modelado
    ├── endpoints.md       # contrato de la API
    └── decisiones.md      # arquitectura y su justificación
```

En el CLAUDE.md quedan las reglas que aplican a todo (arquitectura en capas, convenciones, manejo de errores, verificación) y punteros del tipo "antes de tocar el modelo de datos, leé `docs/modelo-datos.md`".

Agregá una sección de Testing: los services se entregan siempre con su archivo \_test.go, mocks a mano sin librerías de generación, sin conexión real a Mongo, table-driven. Repositories y models no se testean. El comando de verificación pasa a ser go build ./... && go vet ./... && go test ./....
