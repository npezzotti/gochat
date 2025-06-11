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
        <button id="close-btn" className="icon-button" aria-label="Close">
          <FontAwesomeIcon icon={faX} onClick={hideRoomInfoPanel} />
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
            {copyDisabled ?
                <FontAwesomeIcon icon={faFileCircleCheck} />
              :
              <button className="icon-button"  >
                <FontAwesomeIcon icon={faCopy} onClick={changeCopyColor}/>
              </button>
            }
            {copyDisabled ?
              <p>  Copied!</p> :
              <p>  {currentRoom.external_id}</p>
            }
          </div>
          <div className="subscribers">
            <h3>Subscribers</h3>
            <ul className="subscribers-list">
              {currentRoom.subscribers.map((subscriber) => {
                return <li key={subscriber.user_id} className={subscriber.isPresent ? 'status-online' : 'status-offline'} data-user-id={subscriber.user_id} >{subscriber.username}</li>
              })}
            </ul>
          </div>
        </>
      }
    </div>
  );
}