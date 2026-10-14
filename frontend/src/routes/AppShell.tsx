// Layout do cockpit: rail à esquerda (64px reservados) + coluna de conteúdo.
// A TimerBar global entra no topo da coluna (T-13); a fase 03 liga o
// runtime do timer aqui (load inicial restaura o estado no reload — CA-37;
// tick de 1s + re-sync no visibilitychange — CA-37.1).
import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Outlet } from 'react-router-dom';
import SideNav from '../components/SideNav';
import MobileNav from '../components/nav/MobileNav';
import NoticeBanner from '../components/NoticeBanner';
import TimerBar from '../components/timer/TimerBar';
import Spotlight from '../components/spotlight/Spotlight';
import { useSpotlightHotkey } from '../components/spotlight/useSpotlightHotkey';
import type { PomodoroSettings } from '../lib/api';
import { pomodoroKey } from '../lib/queries/settings';
import { attachTimerSync, type AutoStartPolicy } from '../lib/timerStore';

export default function AppShell() {
  const qc = useQueryClient();
  // política de auto-start/som lida do pomodoro_settings cacheado (fase 15,
  // plan D-04): sem cache → default true/true/false (preserva a fase 03).
  useEffect(() => {
    const getPolicy = (): AutoStartPolicy => {
      const s = qc.getQueryData<PomodoroSettings>(pomodoroKey);
      return s
        ? { break: s.auto_start_break, focus: s.auto_start_focus, sound: s.sound_enabled }
        : { break: true, focus: true, sound: false };
    };
    return attachTimerSync(getPolicy);
  }, [qc]);
  // fase 27: atalho global Cmd/Ctrl+K registrado UMA vez no shell (CA-01).
  useSpotlightHotkey();
  return (
    <div className="tt" style={{ display: 'flex', minHeight: '100vh' }}>
      {/* fase 27: o conteúdo do Spotlight (popup central) mora no shell — open=false ⇒ null.
          O BOTÃO de abrir (⌘K) vive no chrome de nav (rail no desktop / menu "Mais" na
          MobileNav — CA-02/03, D-06) e NÃO como overlay fixed: um overlay top-right cobria
          os controles do topo (Iniciar da TimerBar, Dispensar do NoticeBanner) e os tornava
          inclicáveis — regressão pega no gate E2E. */}
      <Spotlight />
      <SideNav />
      <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <NoticeBanner />
        <TimerBar />
        {/* fase 22: o padding-bottom NÃO pode vir do shorthand inline `padding`
            (estilo inline vence a regra .tt-mainpad do @media e a derrota — o
            respiro mobile sumiria e a bottom bar cobriria o conteúdo). Inline só
            top/right/left; o padding-bottom é da classe .tt-mainpad (base
            var(--pad) no desktop, calc(bar+safe-area) < md — tokens.css). */}
        <main
          className="tt-mainpad"
          style={{ flex: 1, minWidth: 0, paddingTop: 'var(--pad)', paddingRight: 'var(--pad)', paddingLeft: 'var(--pad)' }}
        >
          <Outlet />
        </main>
      </div>
      <MobileNav />
    </div>
  );
}
