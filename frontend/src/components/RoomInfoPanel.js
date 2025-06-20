import { useState } from 'react';

import { FontAwesomeIcon } from '@fortawesome/react-fontawesome'
import { faX, faCopy, faFileCircleCheck } from '@fortawesome/free-solid-svg-icons'


export default function RoomInfoPanel({ visible, hideRoomInfoPanel, currentRoom }) {
  const [copyDisabled, setCopyDisabled] = useState(false);

  const changeCopyColor = async (e) => {
    if (copyDisabled) {
      return;
    }

    setCopyDisabled(true);

    try {
      await navigator.clipboard.writeText(currentRoom.external_id);
    } catch (err) {
      console.error('Failed to copy: ', err);
      setCopyDisabled(false);
      return;
    }

    setTimeout(() => {
      setCopyDisabled(false);
    }, 1000);
  }

  return (
    <div className='sidebar' style={{ display: visible ? 'block' : 'none' }}>
      <div className="close-header">
        <button id="close-btn" className="icon-button" aria-label="Close" onClick={hideRoomInfoPanel}>
          <FontAwesomeIcon icon={faX} />
        </button>
      </div>
      {currentRoom &&
        <>
          <div className="room-info">
            <h3>Name</h3>
            <p>{currentRoom.name}</p>
            <h3>Description</h3>
            <p>{currentRoom.description}</p>
            <h3>ID</h3>
            <span style={{ display: 'flex', alignItems: 'center', gap: '0.5em' }}>
              <span>{currentRoom.external_id}</span>
              <button
                className="icon-button"
                {...(!copyDisabled && { onClick: changeCopyColor })}
              >
                {copyDisabled ?
                  <FontAwesomeIcon icon={faFileCircleCheck} />
                  :
                  <FontAwesomeIcon icon={faCopy} />
                }
              </button>
            </span>
          </div>
          <div className="subscribers">
            <h3>Subscribers</h3>
            <ul className="subscribers-list">
              {currentRoom.subscribers.map((subscriber) => {
                return <li key={subscriber.id} className={subscriber.is_present ? 'status-online' : 'status-offline'} data-user-id={subscriber.id} >{subscriber.username}</li>
              })}
            </ul>
          </div>
        </>
      }
    </div>
  );
}