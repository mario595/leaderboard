import { startTransition, useState } from 'react'
import { useNavigate } from 'react-router'
import { deleteAccount } from '../api/api'
import { useAuth } from '../auth/useAuth'

type Step = 'idle' | 'confirm' | 'deleting' | 'failed'

// AccountSection lets the member leave: "Desconectar y borrar mis datos"
// asks for confirmation inline, then DELETE /api/me revokes the app on
// Strava and deletes everything. On success the member lands on
// /account-deleted with the server's confirmation; on failure they stay,
// with an error, and can try again.
export function AccountSection() {
  const auth = useAuth()
  const navigate = useNavigate()
  const [step, setStep] = useState<Step>('idle')

  async function confirm() {
    setStep('deleting')
    try {
      const deletion = await deleteAccount()
      // One transition for both updates. React Router navigates in a
      // transition; forgetting the user on its own would render first, and
      // this protected page would send the member to the login page instead
      // of the confirmation.
      startTransition(() => {
        void navigate('/account-deleted', { replace: true, state: deletion })
        auth.accountDeleted()
      })
    } catch {
      setStep('failed')
    }
  }

  return (
    <section className="account" aria-labelledby="account-title">
      <h2 className="account__title" id="account-title">
        Tu cuenta
      </h2>
      {step === 'idle' ? (
        <>
          <p className="account__text">
            Si te vas, quitamos el acceso de la aplicación a tu Strava y
            borramos tu cuenta y todas tus actividades. Tus actividades en
            Strava no se tocan.
          </p>
          <button
            className="button button--danger"
            type="button"
            onClick={() => setStep('confirm')}
          >
            Desconectar y borrar mis datos
          </button>
        </>
      ) : (
        <div
          className="account__confirm"
          role="alertdialog"
          aria-labelledby="account-confirm-title"
          aria-describedby="account-confirm-text"
        >
          <p className="account__confirm-title" id="account-confirm-title">
            ¿Seguro que quieres borrar tus datos?
          </p>
          <p className="account__text" id="account-confirm-text">
            Desaparecerás de la clasificación y no se puede deshacer. Para
            volver necesitarás una invitación nueva.
          </p>
          {step === 'failed' && (
            <p className="account__error" role="alert">
              No hemos podido borrar tus datos. Inténtalo de nuevo.
            </p>
          )}
          <div className="account__actions">
            <button
              className="button button--danger"
              type="button"
              onClick={() => void confirm()}
              disabled={step === 'deleting'}
            >
              {step === 'deleting' ? 'Borrando…' : 'Sí, borrar mis datos'}
            </button>
            <button
              className="button button--secondary"
              type="button"
              onClick={() => setStep('idle')}
              disabled={step === 'deleting'}
            >
              Cancelar
            </button>
          </div>
        </div>
      )}
    </section>
  )
}
