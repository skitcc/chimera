import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { LikesProvider } from './likes/LikesContext'
import { PlayerProvider } from './player/PlayerContext'
import { Shell } from './components/Shell'
import { AuthPage } from './pages/AuthPage'
import { FeedPage } from './pages/FeedPage'
import { LibraryPage } from './pages/LibraryPage'
import { LikesPage } from './pages/LikesPage'
import { UploadPage } from './pages/UploadPage'
import { ArtistPage } from './pages/ArtistPage'
import { UserTracksPage } from './pages/UserTracksPage'

export function App() {
  return (
    <AuthProvider>
      <LikesProvider>
        <PlayerProvider>
          <BrowserRouter>
            <Routes>
              <Route path="/auth" element={<AuthPage />} />
              <Route element={<Shell />}>
                <Route path="/" element={<FeedPage />} />
                <Route path="/library" element={<LibraryPage />} />
                <Route path="/likes" element={<LikesPage />} />
                <Route path="/upload" element={<UploadPage />} />
                <Route path="/artist" element={<ArtistPage />} />
                <Route path="/u/:id" element={<UserTracksPage />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </PlayerProvider>
      </LikesProvider>
    </AuthProvider>
  )
}
