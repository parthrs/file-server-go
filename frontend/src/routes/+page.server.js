export const load = async () => {
  console.log("Fetching list..")
  const response = await fetch(`http://localhost:37999/list/`);
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
    const upload = await fetch(`http://localhost:37999/upload/${formData.fileToUpload.name}`,
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