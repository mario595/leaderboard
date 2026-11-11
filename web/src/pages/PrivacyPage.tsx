import { PageHeader } from '../components/PageHeader'
import { controllerName, policyUpdated, supportEmail } from '../legal'

// PrivacyPage is the privacy policy (GDPR, and Strava API Policy 6.5 and
// 7.3). Static text: keep it in sync with what the app collects (see
// legal.ts).
export function PrivacyPage() {
  return (
    <>
      <PageHeader
        title="Política de privacidad"
        eyebrow={`Actualizada el ${policyUpdated}`}
      />
      <article className="legal">
        <p>
          Leaderboard (ligadaples.fly.dev) es una aplicación privada, gratuita y
          sin ánimo de lucro con la clasificación de un grupo de entrenamiento.
          Funciona con datos de Strava. Aquí explicamos qué datos tratamos, para
          qué y cómo puedes ejercer tus derechos, según el Reglamento General de
          Protección de Datos (RGPD), el RGPD del Reino Unido y la Ley Orgánica
          3/2018 (LOPDGDD).
        </p>

        <section className="legal__section">
          <h2>Responsable</h2>
          <p>
            {controllerName}, a título personal, administra la aplicación y es
            el responsable del tratamiento. Contacto:{' '}
            <a href={`mailto:${supportEmail}`}>{supportEmail}</a>.
          </p>
        </section>

        <section className="legal__section">
          <h2>Qué datos tratamos</h2>
          <ul>
            <li>
              <strong>De tu cuenta de Strava:</strong> nombre y apellidos, tu
              identificador de atleta y los tokens de acceso que Strava nos da
              al conectar (permisos de lectura de perfil y de actividades).
            </li>
            <li>
              <strong>De cada actividad:</strong> su identificador, el tipo de
              deporte, la fecha y hora local de inicio, el tiempo en movimiento,
              la distancia, el tipo de entrenamiento (por ejemplo «Carrera») y
              la descripción. La descripción solo se lee para buscar la etiqueta{' '}
              <span className="code">#competition</span> y nunca se muestra a
              nadie.
            </li>
            <li>
              <strong>De tu alta:</strong> qué enlace de invitación usaste.
            </li>
            <li>
              <strong>Técnicos:</strong> una cookie de sesión para mantenerte
              conectado y registros del servidor (fecha, página pedida y
              resultado) para detectar fallos. No usamos cookies de publicidad
              ni de analítica.
            </li>
          </ul>
          <p>
            No pedimos tu email ni otros datos de contacto. No tratamos rutas
            GPS, ubicaciones, frecuencia cardiaca ni fotos.
          </p>
        </section>

        <section className="legal__section">
          <h2>Cómo los obtenemos</h2>
          <ul>
            <li>
              Al conectar tu cuenta de Strava (OAuth): tu perfil y los tokens.
            </li>
            <li>
              Al darte de alta importamos tus actividades de la temporada en
              curso.
            </li>
            <li>
              Después, Strava nos avisa cuando creas, cambias o borras una
              actividad, y la volvemos a leer. Una vez al día comparamos tus
              últimos 7 días con Strava para corregir avisos perdidos.
            </li>
          </ul>
        </section>

        <section className="legal__section">
          <h2>Para qué y con qué base legal</h2>
          <p>
            Usamos los datos solo para calcular tus puntos y mostrar la
            clasificación del grupo y tu lista de actividades. No vendemos
            datos, no hacemos publicidad y no los usamos para nada más.
          </p>
          <p>
            La base legal es tu <strong>consentimiento</strong> (artículo 6.1.a
            del RGPD), que das al unirte, en la página de consentimiento que
            aparece antes de conectar tu cuenta de Strava. Puedes retirarlo
            cuando quieras (ver «Cuánto tiempo los guardamos»), sin que afecte a
            lo tratado antes. La cookie de sesión es estrictamente necesaria
            para el servicio que pides.
          </p>
        </section>

        <section className="legal__section">
          <h2>Quién ve tus datos</h2>
          <ul>
            <li>
              Los miembros del grupo ven tu nombre, tus puntos y tu puesto en la
              clasificación.
            </li>
            <li>Tus actividades solo las ves tú.</li>
            <li>
              El administrador puede acceder a la base de datos para mantener la
              aplicación.
            </li>
          </ul>
          <p>
            No cedemos datos a terceros, salvo al proveedor de alojamiento
            (abajo) y cuando lo exija la ley.
          </p>
        </section>

        <section className="legal__section">
          <h2>Dónde se guardan</h2>
          <p>
            La aplicación y su base de datos están alojadas en{' '}
            <strong>Fly.io</strong>, en su región de París (Francia). Fly.io,
            Inc. es una empresa de EE. UU. que actúa como encargado del
            tratamiento: procesa tu dirección IP para servir las páginas y
            guarda la base de datos y sus copias. Si accede a los datos desde
            fuera del Espacio Económico Europeo, lo hace con las garantías de su
            acuerdo de tratamiento de datos (cláusulas contractuales tipo de la
            Comisión Europea).
          </p>
        </section>

        <section className="legal__section">
          <h2>Cuánto tiempo los guardamos</h2>
          <ul>
            <li>Mientras seas miembro del grupo.</li>
            <li>
              Si retiras el acceso a la aplicación en Strava (
              <a
                href="https://www.strava.com/settings/apps"
                target="_blank"
                rel="noreferrer"
              >
                Ajustes → Mis aplicaciones
              </a>
              ), Strava nos avisa y borramos tu cuenta, tus actividades y tus
              tokens. También puedes pedirlo por email.
            </li>
            <li>
              Las copias de seguridad diarias se guardan{' '}
              <strong>14 días</strong>: tus datos desaparecen de ellas como
              mucho 14 días después del borrado.
            </li>
          </ul>
        </section>

        <section className="legal__section">
          <h2>Tus derechos</h2>
          <p>
            Puedes pedir el acceso a tus datos, su rectificación, su supresión,
            la limitación del tratamiento y su portabilidad, y oponerte al
            tratamiento. Escribe a{' '}
            <a href={`mailto:${supportEmail}`}>{supportEmail}</a>: te
            responderemos en un mes como máximo y te confirmaremos el borrado
            cuando lo pidas.
          </p>
          <p>
            Si crees que no tratamos bien tus datos, puedes reclamar ante la
            Agencia Española de Protección de Datos (
            <a href="https://www.aepd.es" target="_blank" rel="noreferrer">
              AEPD
            </a>
            ).
          </p>
        </section>

        <section className="legal__section">
          <h2>Strava</h2>
          <p>
            Strava puede supervisar y recopilar datos de uso de esta aplicación
            (cómo y cuándo usa la API de Strava) y utilizarlos para cualquier
            fin empresarial, interno o externo.
          </p>
          <p>
            Esta política no modifica ni sustituye la{' '}
            <a
              href="https://www.strava.com/legal/privacy"
              target="_blank"
              rel="noreferrer"
            >
              Política de privacidad de Strava
            </a>
            , que prevalece en caso de conflicto.
          </p>
        </section>

        <section className="legal__section">
          <h2>Cambios</h2>
          <p>
            Si cambian los datos que tratamos, te avisaremos y te pediremos de
            nuevo tu consentimiento antes de tratarlos.
          </p>
        </section>
      </article>
    </>
  )
}
