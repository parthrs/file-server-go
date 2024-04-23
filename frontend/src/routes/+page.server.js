export const load = async () => {
  console.log("Fetching list..")
  const response = await fetch(`http://bkend-svc.default.svc.cluster.local:37899/list/`);
  const responseText = await response.text();

  console.log("Returning data..")
  return {
      data: responseText
  };
};

export const actions = {
  default: async ({ request }) => {
    const formData = Object.fromEntries(await request.formData());
    console.log('File:', formData)
    const upload = await fetch(`http://bkend-svc.default.svc.cluster.local:37899/upload/${formData.fileToUpload.name}`,
    {
      method: 'POST',
      body: formData.fileToUpload,
    }).then((response) => response.text()).then((result) => {
      console.log('Success:', result);
      return {
        success: true
      };
    }).catch((error) => {
        console.error('Error:', error);
        return {
          success: false
        };
    });
  }
}