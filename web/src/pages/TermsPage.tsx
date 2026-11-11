import { Link } from 'react-router'
import { PageHeader } from '../components/PageHeader'
import { controllerName, policyUpdated, supportEmail } from '../legal'

// TermsPage is the terms of use (Strava API Policy 9.1 and 9.2). Static text.
export function TermsPage() {
  return (
    <>
      <PageHeader
        title="Términos de uso"
        eyebrow={`Actualizados el ${policyUpdated}`}
      />
      <article className="legal">
        <section className="legal__section">
          <h2>Qué es</h2>
          <p>
            Leaderboard es una aplicación privada, gratuita y sin ánimo de lucro
            para un grupo de entrenamiento, administrada por {controllerName}.
            Solo pueden usarla las personas invitadas.
          </p>
        </section>

        <section className="legal__section">
          <h2>Strava</h2>
          <p>
            Necesitas una cuenta de Strava y usarla según las{' '}
            <a
              href="https://www.strava.com/legal/terms"
              target="_blank"
              rel="noreferrer"
            >
              Condiciones de Strava
            </a>
            . Estos términos no modifican ni contradicen las condiciones de
            Strava ni su acuerdo de API; si alguna vez lo hicieran, prevalecen
            las de Strava.
          </p>
        </section>

        <section className="legal__section">
          <h2>Uso</h2>
          <p>
            Los puntos se calculan con las reglas de la aplicación a partir de
            tus actividades de Strava. Juega limpio: no registres actividades
            que no has hecho. El administrador puede retirar el acceso a quien
            no lo haga.
          </p>
        </section>

        <section className="legal__section">
          <h2>Sin garantías</h2>
          <p>
            La aplicación se ofrece «tal cual» y «según disponibilidad», sin
            garantías de ningún tipo. Puede fallar, tener errores en los puntos
            o dejar de existir. La clasificación no tiene valor oficial.
          </p>
        </section>

        <section className="legal__section">
          <h2>Proveedores de terceros</h2>
          <p>
            En nombre de los proveedores de servicios de terceros (entre ellos
            Strava y Fly.io), quedan excluidas todas las garantías, expresas o
            implícitas, incluidas las garantías implícitas de comerciabilidad,
            de idoneidad para un fin determinado y de no infracción. Los
            proveedores de servicios de terceros no serán responsables de ningún
            daño indirecto, consecuente, especial o punitivo.
          </p>
        </section>

        <section className="legal__section">
          <h2>Responsabilidad</h2>
          <p>
            En la medida en que lo permita la ley, el administrador no responde
            de los daños derivados del uso de la aplicación, salvo en caso de
            dolo o negligencia grave.
          </p>
        </section>

        <section className="legal__section">
          <h2>Baja</h2>
          <p>
            Puedes dejar de usar la aplicación cuando quieras retirándole el
            acceso en Strava. Qué pasa entonces con tus datos está en la{' '}
            <Link to="/privacy">política de privacidad</Link>.
          </p>
        </section>

        <section className="legal__section">
          <h2>Cambios y ley aplicable</h2>
          <p>
            Si estos términos cambian, lo avisaremos en la aplicación. Se rigen
            por la ley española.
          </p>
        </section>

        <section className="legal__section">
          <h2>Contacto</h2>
          <p>
            <a href={`mailto:${supportEmail}`}>{supportEmail}</a>
          </p>
        </section>
      </article>
    </>
  )
}
